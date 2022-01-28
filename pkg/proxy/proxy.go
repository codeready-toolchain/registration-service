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
	"github.com/codeready-toolchain/registration-service/pkg/proxy/namespace"
	"github.com/codeready-toolchain/toolchain-common/pkg/cluster"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
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
)

type Proxy struct {
	namespaces  *UserNamespaces
	tokenParser *auth.TokenParser
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
	return &Proxy{
		namespaces:  NewUserNamespaces(app),
		tokenParser: tokenParser,
	}, nil
}

func (p *Proxy) StartProxy() *http.Server {
	// start server
	mux := http.NewServeMux()
	mux.HandleFunc("/", p.handleRequestAndRedirect)
	mux.HandleFunc("/proxyhealth", p.health)

	// Insert the CORS preflight middleware
	handler := corsPreflightHandler(mux)

	// listen concurrently to allow for graceful shutdown
	log.Info(nil, "Starting the Proxy server...")
	srv := &http.Server{Addr: ":" + ProxyPort, Handler: handler}
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Error(nil, err, err.Error())
		}
	}()
	return srv
}

func (p *Proxy) health(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	_, err := io.WriteString(res, `{"alive": true}`)
	if err != nil {
		log.Error(nil, err, "failed to write health response")
	}
}

func (p *Proxy) handleRequestAndRedirect(res http.ResponseWriter, req *http.Request) {
	ctx, err := p.createContext(req)
	if err != nil {
		log.Error(nil, err, "invalid bearer token")
		responseWithError(res, crterrors.NewUnauthorizedError("invalid bearer token", err.Error()))
		return
	}
	ns, err := p.getTargetNamespace(ctx)
	if err != nil {
		log.Error(ctx, err, "unable to get target namespace")
		responseWithError(res, crterrors.NewInternalError(errors.New("unable to get target namespace"), err.Error()))
		return
	}

	// Note that ServeHttp is non blocking and uses a go routine under the hood
	p.newReverseProxy(ctx, ns).ServeHTTP(res, req)
}

func responseWithError(res http.ResponseWriter, err *crterrors.Error) {
	http.Error(res, err.Error(), err.Code)
}

// createContext creates a new gin.Context with the User ID extracted from the Bearer token.
// To be used for storing the user ID and logging only.
func (p *Proxy) createContext(req *http.Request) (*gin.Context, error) {
	userID, err := p.extractUserID(req)
	if err != nil {
		return nil, err
	}
	keys := make(map[string]interface{})
	keys[context.SubKey] = userID
	return &gin.Context{
		Keys: keys,
	}, nil
}

func (p *Proxy) getTargetNamespace(ctx *gin.Context) (*namespace.NamespaceAccess, error) {
	userID := ctx.GetString(context.SubKey)
	return p.namespaces.GetNamespace(ctx, userID)
}

func (p *Proxy) extractUserID(req *http.Request) (string, error) {
	userToken := ""
	var err error
	if wsstream.IsWebSocketRequest(req) {
		userToken, err = extractTokenFromWebsocketRequest(req)
		if err != nil {
			return "", err
		}
	} else {
		userToken, err = extractUserToken(req)
		if err != nil {
			return "", err
		}
	}

	token, err := p.tokenParser.FromString(userToken)
	if err != nil {
		return "", crterrors.NewUnauthorizedError("unable to extract userID from token", err.Error())
	}
	return token.Subject, nil
}

func extractUserToken(req *http.Request) (string, error) {
	a := req.Header.Get("Authorization")
	token := strings.Split(a, "Bearer ")
	if len(token) < 2 {
		return "", crterrors.NewUnauthorizedError("no token found", "a Bearer token is expected")
	}
	return token[1], nil
}

func (p *Proxy) newReverseProxy(ctx *gin.Context, target *namespace.NamespaceAccess) *httputil.ReverseProxy {
	targetQuery := target.APIURL.RawQuery
	director := func(req *http.Request) {
		origin := req.URL.String()
		req.URL.Scheme = target.APIURL.Scheme
		req.URL.Host = target.APIURL.Host
		req.URL.Path = singleJoiningSlash(target.APIURL.Path, req.URL.Path)
		log.Info(ctx, fmt.Sprintf("forwarding %s to %s", origin, req.URL.String()))
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
			replaceTokenInWebsocketRequest(req, target.SAToken)
		} else {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", target.SAToken))
		}
	}
	transport := http.DefaultTransport
	if !configuration.GetRegistrationServiceConfig().IsProdEnvironment() {
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // nolint:gosec
			},
		}
	}
	return &httputil.ReverseProxy{
		Director:       director,
		Transport:      transport,
		FlushInterval:  -1,
		ModifyResponse: addCorsToResponse,
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
		return nil, errors.Wrap(err, "cannot create ToolchainCluster client")
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
				return "", errors.New("multiple base64.bearer.authorization tokens specified")
			}
			sawTokenProtocol = true

			encodedToken := strings.TrimPrefix(protocol, bearerProtocolPrefix)
			decodedToken, err := base64.RawURLEncoding.DecodeString(encodedToken)
			if err != nil {
				return "", errors.Wrap(err, "invalid base64.bearer.authorization token encoding")
			}
			if !utf8.Valid(decodedToken) {
				return "", errors.New("invalid base64.bearer.authorization token: contains non UTF-8-encoded runes")
			}
			token = string(decodedToken)
		}
	}

	if len(token) == 0 {
		return "", errors.New("no base64.bearer.authorization token found")
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
