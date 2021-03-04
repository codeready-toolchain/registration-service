package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/codeready-toolchain/registration-service/pkg/application"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	clusterproxy "github.com/codeready-toolchain/registration-service/pkg/proxy"
)

// These constant is used to define server
const (
	ProxyPort = "8081"
)

type proxy struct {
	clusters *clusterproxy.UserClusters
}

func newProxy(app application.Application) *proxy {
	return &proxy{
		clusters: clusterproxy.NewUserClusters(app),
	}
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

// Serve a reverse proxy
func (p *proxy) serveReverseProxy(target string, res http.ResponseWriter, req *http.Request) {
	u, _ := url.Parse(target)

	proxy := p.newReverseProxy(u)

	// Note that ServeHttp is non blocking and uses a go routine under the hood
	proxy.ServeHTTP(res, req)
}

func (p *proxy) getProxyURL(req *http.Request) string {
	auth := req.Header.Get("Authorization")
	token := strings.Split(auth, "Bearer ")
	if len(token) < 2 {
		// TODO return the first/random cluster URL
		return ""
	}
	bearer := token[1]
	url, err := p.clusters.Url(bearer)
	if err != nil {
		log.Error(nil, err, "unable to get cluster url by token")
		// TODO return the first/random cluster URL or 401 with the message about expired token
		return ""
	}
	return url
}

// Given a request send it to the appropriate url
func (p *proxy) handleRequestAndRedirect(res http.ResponseWriter, req *http.Request) {
	u := p.getProxyURL(req)
	log.Info(nil, fmt.Sprintf("proxy url: %s", u))

	p.serveReverseProxy(u, res, req)
}

func (p *proxy) newReverseProxy(target *url.URL) *httputil.ReverseProxy {
	targetQuery := target.RawQuery
	director := func(req *http.Request) {
		req.URL.Scheme = "https" // Always use https
		req.URL.Host = target.Host
		req.URL.Path = singleJoiningSlash(target.Path, req.URL.Path)
		if targetQuery == "" || req.URL.RawQuery == "" {
			req.URL.RawQuery = targetQuery + req.URL.RawQuery
		} else {
			req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
		}
		if _, ok := req.Header["User-Agent"]; !ok {
			// explicitly disable User-Agent so it's not set to default value
			req.Header.Set("User-Agent", "")
		}
	}
	return &httputil.ReverseProxy{Director: director}
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
