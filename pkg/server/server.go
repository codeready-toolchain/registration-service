package server

import (
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	errs "github.com/pkg/errors"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/controller"
)

// RegistrationServer bundles configuration, logging, and HTTP server objects in a single
// location.
type RegistrationServer struct {
	config            *configuration.Registry
	router            *gin.Engine
	httpServer        *http.Server
	logger            *log.Logger
	websocketsHandler *controller.WebsocketsHandler
	routesSetup       sync.Once
}

// New creates a new RegistrationServer object with reasonable defaults.
func New(configFilePath string) (*RegistrationServer, error) {
	srv := &RegistrationServer{
		router: gin.Default(),
		logger: log.New(os.Stdout, "", 0),
	}
	gin.DefaultWriter = io.MultiWriter(os.Stdout)

	config, err := configuration.New(configFilePath)
	if err != nil {
		return nil, errs.Wrapf(err, "failed to create a new configuration registry from file %q", configFilePath)
	}
	srv.config = config

	srv.httpServer = &http.Server{
		Addr: srv.config.GetHTTPAddress(),
		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout: srv.config.GetHTTPWriteTimeout(),
		ReadTimeout:  srv.config.GetHTTPReadTimeout(),
		IdleTimeout:  srv.config.GetHTTPIdleTimeout(),
		Handler:      srv.router,
	}
	if srv.config.GetHTTPCompressResponses() {
		srv.router.Use(gzip.Gzip(gzip.DefaultCompression))
	}
	return srv, nil
}

// UseWebsocketsHandler registers the given hub for websockets connections.
func (srv *RegistrationServer) UseWebsocketsHandler(handler *controller.WebsocketsHandler) error {
	if handler == nil {
		return errors.New("given WebsocketsHandler is nil")
	}
	srv.websocketsHandler = handler
	return nil
}

// WebsocketsHandler returns the websocket handler instance.
func (srv *RegistrationServer) WebsocketsHandler() *controller.WebsocketsHandler {
	return srv.websocketsHandler
}

// Logger returns the app server's log object.
func (srv *RegistrationServer) Logger() *log.Logger {
	return srv.logger
}

// Config returns the app server's config object.
func (srv *RegistrationServer) Config() *configuration.Registry {
	return srv.config
}

// HTTPServer returns the app server's HTTP server.
func (srv *RegistrationServer) HTTPServer() *http.Server {
	return srv.httpServer
}

// Engine returns the app server's HTTP router.
func (srv *RegistrationServer) Engine() *gin.Engine {
	return srv.router
}

// GetRegisteredRoutes returns all registered routes formatted with their
// methods, paths, queries and names. It is a good idea to print this
// information on server start to give you an idea of what routes are
// available in the system.
func (srv *RegistrationServer) GetRegisteredRoutes() string {
	var sb strings.Builder

	for _, routeInfo := range srv.router.Routes() {
		sb.WriteString("ROUTE: ")
		sb.WriteString("\tRoute Path: ")
		sb.WriteString(routeInfo.Path)
		sb.WriteString("\n\tMethod: ")
		sb.WriteString(routeInfo.Method)
		sb.WriteString("\n")
	}
	return sb.String()
}
