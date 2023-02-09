package proxy

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/textproto"
	"strings"
	"time"
	"unicode/utf8"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/application"
	"github.com/codeready-toolchain/registration-service/pkg/auth"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	crterrors "github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/access"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/handlers"
	"github.com/codeready-toolchain/toolchain-common/pkg/cluster"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	glog "github.com/labstack/gommon/log"
	errs "github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/apiserver/pkg/util/wsstream"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerlog "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	ProxyPort            = "8081"
	bearerProtocolPrefix = "base64url.bearer.authorization.k8s.io." //nolint:gosec

	proxyHealthEndpoint = "/proxyhealth"
)

type Proxy struct {
	app         application.Application
	cl          client.Client
	tokenParser *auth.TokenParser
	spaceLister *handlers.SpaceLister
}

func NewProxy(app application.Application) (*Proxy, error) {
	cl, err := newClusterClient()
	if err != nil {
		return nil, err
	}
	return newProxyWithClusterClient(app, cl)
}

func newProxyWithClusterClient(app application.Application, cln client.Client) (*Proxy, error) {
	// Initiate toolchain cluster cache service
	cacheLog := controllerlog.Log.WithName("registration-service")
	cluster.NewToolchainClusterService(cln, cacheLog, configuration.Namespace(), 5*time.Second)

	tokenParser, err := auth.DefaultTokenParser()
	if err != nil {
		return nil, err
	}

	// init handlers
	spaceLister := handlers.NewSpaceLister(app)

	return &Proxy{
		app:         app,
		cl:          cln,
		tokenParser: tokenParser,
		spaceLister: spaceLister,
	}, nil
}

func (p *Proxy) StartProxy() *http.Server {
	// start server
	router := echo.New()
	router.Logger.SetLevel(glog.INFO)

	// middleware before routing
	router.Pre(
		middleware.RemoveTrailingSlash(),
		p.addUserContext(), // get user information from token before handling request
	)

	// middleware after routing
	router.Use(
		middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
			Skipper: func(ctx echo.Context) bool {
				return ctx.Request().URL.RequestURI() == proxyHealthEndpoint // skip logging for health check so it doesn't pollute the logs
			},
			LogStatus: true,
			LogURI:    true,
			LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
				fmt.Printf("REQUEST: uri: %v, status: %v\n", v.URI, v.Status)
				return nil
			},
		}),
		middleware.Recover(),
	)

	// routes
	wg := router.Group("/apis/toolchain.dev.openshift.com/v1alpha1/workspaces")
	wg.GET("/:workspace", p.spaceLister.HandleSpaceListRequest)
	wg.GET("", p.spaceLister.HandleSpaceListRequest)

	router.GET(proxyHealthEndpoint, p.health)
	router.Any("/*", p.handleRequestAndRedirect)

	// Insert the CORS preflight middleware
	handler := corsPreflightHandler(router)

	log.Info(nil, "Starting the Proxy server...")
	srv := &http.Server{Addr: ":" + ProxyPort, Handler: handler, ReadHeaderTimeout: 2 * time.Second}
	// listen concurrently to allow for graceful shutdown
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Error(nil, err, err.Error())
		}
	}()

	return srv
}

func (p *Proxy) health(ctx echo.Context) error {
	ctx.Response().Writer.Header().Set("Content-Type", "application/json")
	ctx.Response().Writer.WriteHeader(http.StatusOK)
	_, err := io.WriteString(ctx.Response().Writer, `{"alive": true}`)
	return err
}

func (p *Proxy) handleRequestAndRedirect(ctx echo.Context) error {
	userID, _ := ctx.Get(context.SubKey).(string)
	username, _ := ctx.Get(context.UsernameKey).(string)

	workspace, err := handleWorkspaceContext(ctx.Request())
	if err != nil {
		ctx.Logger().Error("unable to get workspace context", err)
		responseWithError(ctx.Response().Writer, crterrors.NewInternalError(errs.New("unable to get target cluster"), err.Error()))
		return nil
	}

	cluster, err := p.app.MemberClusterService().GetClusterAccess(userID, username, workspace)
	if err != nil {
		log.Error(nil, err, "unable to get target cluster")
		responseWithError(ctx.Response().Writer, crterrors.NewInternalError(errs.New("unable to get target cluster"), err.Error()))
		return nil
	}

	// Note that ServeHttp is non blocking and uses a go routine under the hood
	p.newReverseProxy(ctx, ctx.Request(), cluster).ServeHTTP(ctx.Response().Writer, ctx.Request())
	return nil
}

func handleWorkspaceContext(req *http.Request) (string, error) {
	path := req.URL.Path
	var workspace string
	// handle specific workspace request eg. /workspaces/mycoolworkspace/api/pods
	if strings.HasPrefix(path, "/workspaces/") {
		segments := strings.Split(path, "/")
		// there should be at least 4 segments eg. /workspaces/mycoolworkspace/api counts as 4
		if len(segments) < 4 {
			return "", fmt.Errorf("workspace request path has too few segments '%s'; expected path format: /workspaces/<workspace_name>/api/...", path) // nolint:revive
		}
		// get the workspace segment eg. mycoolworkspace
		workspace = segments[2]
		// remove workspaces/mycoolworkspace from the request path before forwarding the request
		req.URL.Path = strings.TrimPrefix(req.URL.Path, "/workspaces/"+workspace)
	}
	return workspace, nil
}

func responseWithError(res http.ResponseWriter, err *crterrors.Error) {
	http.Error(res, err.Error(), err.Code)
}

// addUserContext updates echo.Context with the User ID extracted from the Bearer token.
// To be used for storing the user ID and logging only.
func (p *Proxy) addUserContext() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			if ctx.Request().URL.Path == proxyHealthEndpoint { // skip only for health endpoint
				return next(ctx)
			}

			userID, username, err := p.extractUserID(ctx.Request())
			if err != nil {
				ctx.Logger().Error(err) // log the original error
				responseWithError(ctx.Response().Writer, crterrors.NewUnauthorizedError("invalid bearer token", err.Error()))
				return nil
			}
			ctx.Set(context.SubKey, userID)
			ctx.Set(context.UsernameKey, username)

			return next(ctx)
		}
	}
}

func (p *Proxy) extractUserID(req *http.Request) (string, string, error) {
	userToken := ""
	var err error
	if wsstream.IsWebSocketRequest(req) {
		userToken, err = extractTokenFromWebsocketRequest(req)
		if err != nil {
			return "", "", err
		}
	} else {
		userToken, err = extractUserToken(req)
		if err != nil {
			return "", "", err
		}
	}

	token, err := p.tokenParser.FromString(userToken)
	if err != nil {
		return "", "", crterrors.NewUnauthorizedError("unable to extract userID from token", err.Error())
	}
	return token.Subject, token.PreferredUsername, nil
}

func extractUserToken(req *http.Request) (string, error) {
	a := req.Header.Get("Authorization")
	token := strings.Split(a, "Bearer ")
	if len(token) < 2 {
		return "", crterrors.NewUnauthorizedError("no token found", "a Bearer token is expected")
	}
	return token[1], nil
}

func (p *Proxy) newReverseProxy(ctx echo.Context, req *http.Request, target *access.ClusterAccess) *httputil.ReverseProxy {
	targetQuery := target.APIURL().RawQuery
	director := func(req *http.Request) {
		origin := req.URL.String()
		req.URL.Scheme = target.APIURL().Scheme
		req.URL.Host = target.APIURL().Host
		req.URL.Path = singleJoiningSlash(target.APIURL().Path, req.URL.Path)
		ctx.Logger().Info(fmt.Sprintf("forwarding %s to %s", origin, req.URL.String()))
		if targetQuery == "" || req.URL.RawQuery == "" {
			req.URL.RawQuery = targetQuery + req.URL.RawQuery
		} else {
			req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
		}
		if _, ok := req.Header["User-Agent"]; !ok {
			// explicitly disable User-Agent so it's not set to default value
			req.Header.Set("User-Agent", "")
		}
		// Replace token
		if wsstream.IsWebSocketRequest(req) {
			replaceTokenInWebsocketRequest(req, target.ImpersonatorToken())
		} else {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", target.ImpersonatorToken()))
		}

		// Set impersonation header
		req.Header.Set("Impersonate-User", target.Username())
	}
	transport := http.DefaultTransport
	if !configuration.GetRegistrationServiceConfig().IsProdEnvironment() {
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // nolint:gosec
			},
		}
	}
	m := &responseModifier{req.Header.Get("Origin")}
	return &httputil.ReverseProxy{
		Director:       director,
		Transport:      transport,
		FlushInterval:  -1,
		ModifyResponse: m.addCorsToResponse,
	}
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}

func newClusterClient() (client.Client, error) {
	scheme := runtime.NewScheme()
	if err := v1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := toolchainv1alpha1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	cl, err := client.New(k8sConfig, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, errs.Wrap(err, "cannot create ToolchainCluster client")
	}
	return cl, nil
}

var ph = textproto.CanonicalMIMEHeaderKey("Sec-WebSocket-Protocol")

func extractTokenFromWebsocketRequest(req *http.Request) (string, error) {
	token := ""
	sawTokenProtocol := false
	for _, protocolHeader := range req.Header[ph] {
		for _, protocol := range strings.Split(protocolHeader, ",") {
			protocol = strings.TrimSpace(protocol)
			if !strings.HasPrefix(protocol, bearerProtocolPrefix) {
				continue
			}

			if sawTokenProtocol {
				return "", errs.New("multiple base64.bearer.authorization tokens specified")
			}
			sawTokenProtocol = true

			encodedToken := strings.TrimPrefix(protocol, bearerProtocolPrefix)
			decodedToken, err := base64.RawURLEncoding.DecodeString(encodedToken)
			if err != nil {
				return "", errs.Wrap(err, "invalid base64.bearer.authorization token encoding")
			}
			if !utf8.Valid(decodedToken) {
				return "", errs.New("invalid base64.bearer.authorization token: contains non UTF-8-encoded runes")
			}
			token = string(decodedToken)
		}
	}

	if len(token) == 0 {
		return "", errs.New("no base64.bearer.authorization token found")
	}

	return token, nil
}

func replaceTokenInWebsocketRequest(req *http.Request, newToken string) {
	var protocols []string
	encodedToken := base64.RawURLEncoding.EncodeToString([]byte(newToken))
	for _, protocolHeader := range req.Header[ph] {
		for _, protocol := range strings.Split(protocolHeader, ",") {
			protocol = strings.TrimSpace(protocol)
			if !strings.HasPrefix(protocol, bearerProtocolPrefix) {
				protocols = append(protocols, protocol)
				continue
			}
			// Replace the token
			protocols = append(protocols, bearerProtocolPrefix+encodedToken)
		}
	}
	req.Header.Set(ph, strings.Join(protocols, ","))
}
