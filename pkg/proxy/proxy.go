package proxy

import (
	"crypto/tls"
	"encoding/base64"
	"errors"
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
	"k8s.io/apimachinery/pkg/util/httpstream"

	"k8s.io/apiserver/pkg/util/wsstream"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerlog "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	ProxyPort            = "8081"
	bearerProtocolPrefix = "base64url.bearer.authorization.k8s.io." //nolint:gosec

	proxyHealthEndpoint = "/proxyhealth"

	pluginsEndpoint = "/plugins/"

	workspaceCtxKey = "workspace"
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
	router.HTTPErrorHandler = customHTTPErrorHandler

	// middleware before routing
	router.Pre(
		middleware.RemoveTrailingSlash(),
		p.stripInvalidHeaders(),
		p.addUserContext(), // get user information from token before handling request
	)

	// middleware after routing
	router.Use(
		middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
			Skipper: func(ctx echo.Context) bool {
				return ctx.Request().URL.RequestURI() == proxyHealthEndpoint // skip logging for health check so it doesn't pollute the logs
			},
			LogMethod: true,
			LogStatus: true,
			LogURI:    true,
			LogValuesFunc: func(ctx echo.Context, v middleware.RequestLoggerValues) error {
				userID, _ := ctx.Get(context.SubKey).(string)
				username, _ := ctx.Get(context.UsernameKey).(string)
				workspace, _ := ctx.Get(workspaceCtxKey).(string)

				fmt.Printf("REQUEST: method: %v, uri: %v, workspace: %v, userID: %v, username: %v\n", v.Method, v.URI, workspace, userID, username)
				return nil
			},
		}),
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

	proxyPluginName, workspace, err := getWorkspaceContext(ctx.Request())
	if err != nil {
		return crterrors.NewBadRequest("unable to get workspace context", err.Error())
	}
	ctx.Set(workspaceCtxKey, workspace) // set workspace context for logging

	cluster, err := p.app.MemberClusterService().GetClusterAccess(userID, username, workspace, proxyPluginName)
	if err != nil {
		return crterrors.NewInternalError(errs.New("unable to get target cluster"), err.Error())
	}

	// before proxying the request, verify that the user has a spacebinding for the workspace and that the namespace (if any) belongs to the workspace
	workspaces, err := p.spaceLister.ListUserWorkspaces(ctx)
	if err != nil {
		return crterrors.NewInternalError(errs.New("unable to retrieve user workspaces"), err.Error())
	}

	requestedNamespace := namespaceFromCtx(ctx)
	if err := validateWorkspaceRequest(workspace, requestedNamespace, workspaces); err != nil {
		return crterrors.NewForbiddenError("invalid workspace request", err.Error())
	}

	// Note that ServeHttp is non blocking and uses a go routine under the hood
	p.newReverseProxy(ctx, cluster, len(proxyPluginName) > 0).ServeHTTP(ctx.Response().Writer, ctx.Request())
	return nil
}

func getWorkspaceContext(req *http.Request) (string, string, error) {
	path := req.URL.Path
	proxyPluginName := ""
	// first string off any preceding proxy plugin url segment
	if strings.HasPrefix(path, pluginsEndpoint) {
		segments := strings.Split(path, "/")
		// NOTE: a split on "/plugins/" results in an array with 2 items.  One is the empty string, two is "plugins", 3 is empty string
		// behavior is not unique to "/";  "," and ",plugins," works the same way
		if len(segments) < 3 {
			return "", "", fmt.Errorf("path %q not a proxied route request", path)
		}
		if len(segments) == 3 {
			// need to distinguish between the third entry being "" vs "<plugin-name>"
			if len(strings.TrimSpace(segments[2])) == 0 {
				return "", "", fmt.Errorf("path %q not a proxied route request", path)
			}
		}
		proxyPluginName = segments[2]
		req.URL.Path = strings.TrimPrefix(path, fmt.Sprintf("/%s/%s", segments[1], segments[2]))
		path = req.URL.Path
	}
	var workspace string
	// handle specific workspace request eg. /workspaces/mycoolworkspace/api/clusterroles
	if strings.HasPrefix(path, "/workspaces/") {
		segments := strings.Split(path, "/")
		// there should be at least 4 segments eg. /workspaces/mycoolworkspace/api/clusterroles counts as 4
		if len(segments) < 4 && len(proxyPluginName) == 0 {
			return "", "", fmt.Errorf("workspace request path has too few segments '%s'; expected path format: /workspaces/<workspace_name>/api/...", path) // nolint:revive
		}
		// with proxy plugins, the route host is sufficient, and hence do not need api/...
		if len(segments) < 3 {
			return "", "", fmt.Errorf("workspace request path has too few segments '%s'; expected path format: /workspaces/<workspace_name>/<optional path>", path) // nolint:revive
		}
		if len(segments) == 3 {
			// need to distinguish between the third entry being "" vs "<plugin-name>"
			if len(strings.TrimSpace(segments[2])) == 0 {
				return "", "", fmt.Errorf("workspace request path has too few segments '%s'; expected path format: /workspaces/<workspace_name>/<optional path>", path) // nolint:revive
			}
		}
		// get the workspace segment eg. mycoolworkspace
		workspace = segments[2]
		// remove workspaces/mycoolworkspace from the request path before forwarding the request
		req.URL.Path = strings.TrimPrefix(req.URL.Path, "/workspaces/"+workspace)
	}

	return proxyPluginName, workspace, nil
}

func customHTTPErrorHandler(cause error, ctx echo.Context) {
	code := http.StatusInternalServerError
	ce := &crterrors.Error{}
	if errors.As(cause, &ce) {
		code = ce.Code
	}
	ctx.Logger().Error(cause)
	if err := ctx.String(code, cause.Error()); err != nil {
		ctx.Logger().Error(err)
	}
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
				return crterrors.NewUnauthorizedError("invalid bearer token", err.Error())
			}
			ctx.Set(context.SubKey, userID)
			ctx.Set(context.UsernameKey, username)

			return next(ctx)
		}
	}
}

func (p *Proxy) stripInvalidHeaders() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			for header := range ctx.Request().Header {
				lowercase := strings.ToLower(header)
				if strings.HasPrefix(lowercase, "impersonate-") {
					log.Info(nil, fmt.Sprintf("Removing invalid header %s from context '%+v'", header, ctx))
					ctx.Request().Header.Del(header)
				}
			}
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

func (p *Proxy) newReverseProxy(ctx echo.Context, target *access.ClusterAccess, isPlugin bool) *httputil.ReverseProxy {
	req := ctx.Request()
	targetQuery := target.APIURL().RawQuery
	director := func(req *http.Request) {
		origin := req.URL.String()
		req.URL.Scheme = target.APIURL().Scheme
		req.URL.Host = target.APIURL().Host
		req.URL.Path = singleJoiningSlash(target.APIURL().Path, req.URL.Path)
		if isPlugin {
			// for non k8s clients testing, like vanilla http clients accessing plugin proxy flows, testing has proven that the request
			// host needs to be updated in addition to the URL in order to have the reverse proxy contact the openshift
			// route on the member cluster
			req.Host = target.APIURL().Host
		}
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
	transport := getTransport(req.Header)
	m := &responseModifier{req.Header.Get("Origin")}
	return &httputil.ReverseProxy{
		Director:       director,
		Transport:      transport,
		FlushInterval:  -1,
		ModifyResponse: m.addCorsToResponse,
	}
}

func getTransport(reqHeader http.Header) http.RoundTripper {
	if !configuration.GetRegistrationServiceConfig().IsProdEnvironment() {
		return &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // nolint:gosec
			},
		}
	}
	// for exec and rsh command we cannot use h2 because it doesn't support "Upgrade: SPDY/3.1" header https://github.com/kubernetes/kubernetes/issues/7452
	if strings.HasPrefix(strings.ToLower(reqHeader.Get(httpstream.HeaderUpgrade)), "spdy/") {
		// thus, we need to switch to http/1.1
		transport := http.DefaultTransport.(interface {
			Clone() *http.Transport
		}).Clone()
		transport.ForceAttemptHTTP2 = false
		transport.TLSClientConfig.NextProtos = []string{"http/1.1"}
		return transport
	}
	return http.DefaultTransport
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

func validateWorkspaceRequest(requestedWorkspace, requestedNamespace string, workspaces []toolchainv1alpha1.Workspace) error {
	// check workspace access
	isHomeWSRequested := requestedWorkspace == ""

	allowedWorkspace := -1
	for i, w := range workspaces {
		if w.Name == requestedWorkspace || (isHomeWSRequested && w.Status.Type == "home") {
			allowedWorkspace = i
			break
		}
	}
	if allowedWorkspace == -1 {
		return fmt.Errorf("access to workspace '%s' is forbidden", requestedWorkspace)
	}

	// check namespace access
	if requestedNamespace != "" {
		allowedNamespace := false
		namespaces := workspaces[allowedWorkspace].Status.Namespaces
		for _, ns := range namespaces {
			if ns.Name == requestedNamespace {
				allowedNamespace = true
				break
			}
		}
		if !allowedNamespace {
			return fmt.Errorf("access to namespace '%s' in workspace '%s' is forbidden", requestedNamespace, workspaces[allowedWorkspace].Name)
		}
	}
	return nil
}

func namespaceFromCtx(ctx echo.Context) string {
	path := ctx.Request().URL.Path
	if strings.Index(path, "/namespaces/") > 0 {
		segments := strings.Split(path, "/")
		for i, segment := range segments {
			if segment == "namespaces" && i+1 < len(segments) {
				return segments[i+1]
			}
		}
	}
	return ""
}
