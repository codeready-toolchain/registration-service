package server

import (
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/codeready-toolchain/registration-service/pkg/application"
	"github.com/gin-contrib/cors"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

type ServerOption = func(server *RegistrationServer)

// RegistrationServer bundles configuration, and HTTP server objects in a single
// location.
type RegistrationServer struct {
	config      configuration.Configuration
	router      *gin.Engine
	httpServer  *http.Server
	routesSetup sync.Once
	//applicationProducerFunc func() application.Application
	application application.Application
}

// New creates a new RegistrationServer object with reasonable defaults.
func New(config configuration.Configuration, application application.Application) *RegistrationServer {

	// Disable logging for the /api/v1/health endpoint so that our logs aren't overwhelmed
	ginRouter := gin.New()
	ginRouter.Use(
		gin.LoggerWithWriter(gin.DefaultWriter, "/api/v1/health"),
		gin.Recovery(),
		cors.New(cors.Config{
			AllowAllOrigins:     true,
			AllowMethods:     []string{"PUT", "PATCH", "POST", "GET", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Content-Length", "Content-Type", "Authorization", "Accept"},
			ExposeHeaders:    []string{"Content-Length", "Authorization"},
			AllowCredentials: true,
		}),
	)

	srv := &RegistrationServer{
		router:      ginRouter,
		application: application,
	}

	gin.DefaultWriter = io.MultiWriter(os.Stdout)

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
	return srv
}

// Config returns the app server's config object.
func (srv *RegistrationServer) Config() configuration.Configuration {
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
