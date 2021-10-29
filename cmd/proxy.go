package main

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
	crterrors "github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	clusterproxy "github.com/codeready-toolchain/registration-service/pkg/proxy"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/namespace"
	"github.com/codeready-toolchain/toolchain-common/pkg/cluster"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerlog "sigs.k8s.io/controller-runtime/pkg/log"
)

// These constant is used to define server
const (
	ProxyPort = "8081"
)

type proxy struct {
	namespaces  *clusterproxy.UserNamespaces
	tokenParser *auth.TokenParser
}

func newProxy(app application.Application, config configuration.RegistrationServiceConfig) (*proxy, error) {
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
		namespaces:  clusterproxy.NewUserNamespaces(app),
		tokenParser: tokenParserInstance,
	}, nil
}

func (p *proxy) startProxy() *http.Server {
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

// Given a request send it to the appropriate url
func (p *proxy) handleRequestAndRedirect(res http.ResponseWriter, req *http.Request) {
	ns, err := p.getTargetNamespace(req)
	if err != nil {
		// TODO populate the request with the error
		panic(errors.Wrap(err, "unable to get target namespace"))
	}
	log.Info(nil, fmt.Sprintf("proxy url: %s, namespace: %s", ns.ApiURL.String(), ns.Namespace))

	p.serveReverseProxy(ns, res, req)
}

func (p *proxy) getTargetNamespace(req *http.Request) (*namespace.Namespace, error) {
	userToken, err := extractUserToken(req)
	if err != nil {
		return nil, err
	}
	userID, err := p.extractUserID(userToken)
	if err != nil {
		return nil, err
	}
	return p.namespaces.GetNamespace(userID)
}

func extractUserToken(req *http.Request) (string, error) {
	a := req.Header.Get("Authorization")
	token := strings.Split(a, "Bearer ")
	if len(token) < 2 {
		return "", crterrors.NewUnauthorizedError("no token found", "a Bearer token is expected")
	}
	return token[1], nil
}

func (p *proxy) extractUserID(tokenStr string) (string, error) {
	token, err := p.tokenParser.FromString(tokenStr)
	if err != nil {
		return "", crterrors.NewUnauthorizedError("unable to extract userID from token", err.Error())
	}
	return token.Subject, nil
}

// Serve a reverse proxy
func (p *proxy) serveReverseProxy(target *namespace.Namespace, res http.ResponseWriter, req *http.Request) {
	proxy := p.newReverseProxy(target)

	// Note that ServeHttp is non blocking and uses a go routine under the hood
	proxy.ServeHTTP(res, req)
}

func (p *proxy) newReverseProxy(target *namespace.Namespace) *httputil.ReverseProxy {
	targetQuery := target.ApiURL.RawQuery
	director := func(req *http.Request) {
		origin := req.URL.String()
		req.URL.Scheme = "https" // Always use https
		req.URL.Host = target.ApiURL.Host
		// TODO Replace/insert namespace path
		req.URL.Path = singleJoiningSlash(target.ApiURL.Path, req.URL.Path)
		log.Info(nil, fmt.Sprintf("forwarding %s to %s\n\n", origin, req.URL.String()))
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
