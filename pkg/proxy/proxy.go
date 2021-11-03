package proxy

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

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
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerlog "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	ProxyPort = "8081"
)

type proxy struct {
	namespaces  *UserNamespaces
	tokenParser *auth.TokenParser
}

func NewProxy(app application.Application, config configuration.RegistrationServiceConfig) (*proxy, error) {
	// Initiate toolchain cluster cache service
	cacheLog := controllerlog.Log.WithName("registration-service")
	cl, err := newClusterClient()
	if err != nil {
		return nil, err
	}
	cluster.NewToolchainClusterService(cl, cacheLog, config.Namespace(), 5*time.Second)

	tokenParserInstance, err := auth.DefaultTokenParser()
	if err != nil {
		return nil, err
	}
	return &proxy{
		namespaces:  NewUserNamespaces(app),
		tokenParser: tokenParserInstance,
	}, nil
}

func (p *proxy) StartProxy() *http.Server {
	// start server
	mux := http.NewServeMux()
	mux.HandleFunc("/", p.handleRequestAndRedirect)

	// listen concurrently to allow for graceful shutdown
	log.Info(nil, "Starting the proxy server...")
	srv := &http.Server{Addr: ":" + ProxyPort, Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Error(nil, err, err.Error())
			panic(fmt.Sprintf("Proxy server stoped: %s", err.Error()))
		}
	}()
	return srv
}

func (p *proxy) handleRequestAndRedirect(res http.ResponseWriter, req *http.Request) {
	ctx, err := p.createContext(req)
	if err != nil {
		log.Error(nil, err, "unable to create a context")
		responseWithError(res, crterrors.NewUnauthorizedError("unable to create a context", err.Error()))
		return
	}
	ns, err := p.getTargetNamespace(ctx, req)
	if err != nil {
		log.Error(ctx, err, "unable to get target namespace")
		responseWithError(res, crterrors.NewInternalError(errors.New("unable to get target namespace"), err.Error()))
		return
	}

	p.serveReverseProxy(ctx, ns, res, req)
}

func responseWithError(res http.ResponseWriter, err *crterrors.Error) {
	http.Error(res, err.Error(), err.Code)
}

// createContext creates a new gin.Context with the User ID extracted from the Bearer token.
// To be used for storing the user ID and logging only.
func (p *proxy) createContext(req *http.Request) (*gin.Context, error) {
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

func (p *proxy) getTargetNamespace(ctx *gin.Context, req *http.Request) (*namespace.Namespace, error) {
	userID := ctx.GetString(context.SubKey)
	return p.namespaces.GetNamespace(ctx, userID)
}

func (p *proxy) extractUserID(req *http.Request) (string, error) {
	userToken, err := extractUserToken(req)
	if err != nil {
		return "", err
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

func (p *proxy) serveReverseProxy(ctx *gin.Context, target *namespace.Namespace, res http.ResponseWriter, req *http.Request) {
	proxy := p.newReverseProxy(ctx, target)

	// Note that ServeHttp is non blocking and uses a go routine under the hood
	proxy.ServeHTTP(res, req)
}

func (p *proxy) newReverseProxy(ctx *gin.Context, target *namespace.Namespace) *httputil.ReverseProxy {
	targetQuery := target.ApiURL.RawQuery
	director := func(req *http.Request) {
		origin := req.URL.String()
		req.URL.Scheme = "https" // Always use https
		req.URL.Host = target.ApiURL.Host
		req.URL.Path = singleJoiningSlash(target.ApiURL.Path, req.URL.Path)
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
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", target.TargetClusterToken))
	}
	var transport *http.Transport
	if !configuration.GetRegistrationServiceConfig().IsProdEnvironment() {
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}
	return &httputil.ReverseProxy{
		Director:  director,
		Transport: transport,
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
	toolchainv1alpha1.SchemeBuilder.Register(
		&toolchainv1alpha1.ToolchainCluster{},
		&toolchainv1alpha1.ToolchainClusterList{},
	)

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
