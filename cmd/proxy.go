package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/codeready-toolchain/registration-service/pkg/log"
)

// These constant is used to define server
const (
	ProxyPort = "8081"
)

func startProxy() *http.Server {
	// start server
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleRequestAndRedirect)

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

// Serve a reverse proxy for a given url
func serveReverseProxy(target string, res http.ResponseWriter, req *http.Request) {
	// parse the url
	u, _ := url.Parse(target)

	// create the reverse proxy
	proxy := NewReverseProxy(u)

	// Note that ServeHttp is non blocking and uses a go routine under the hood
	proxy.ServeHTTP(res, req)
}

// Balance returns one of the servers based using round-robin algorithm
func getProxyURL() string {
	//TODO
	return "https://api.sandbox.x8i5.p1.openshiftapps.com:6443"
}

// Given a request send it to the appropriate url
func handleRequestAndRedirect(res http.ResponseWriter, req *http.Request) {
	u := getProxyURL()
	log.Info(nil, fmt.Sprintf("proxy url: %s", u))

	serveReverseProxy(u, res, req)
}

func NewReverseProxy(target *url.URL) *httputil.ReverseProxy {
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
