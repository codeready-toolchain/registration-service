package server

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/application"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

type ServerOption = func(server *RegistrationServer) // nolint: golint

// RegistrationServer bundles configuration, and HTTP server objects in a single
// location.
type RegistrationServer struct {
	router      *gin.Engine
	httpServer  *http.Server
	routesSetup sync.Once
	//applicationProducerFunc func() application.Application
	application application.Application
}

// New creates a new RegistrationServer object with reasonable defaults.
func New(application application.Application) *RegistrationServer {

	gin.SetMode(gin.ReleaseMode)
	ginRouter := gin.New()
	ginRouter.Use(
		gin.LoggerWithConfig(gin.LoggerConfig{
			Output:    gin.DefaultWriter,
			SkipPaths: []string{"/api/v1/health"}, // disable logging for the /api/v1/health endpoint so that our logs aren't overwhelmed
			Formatter: func(params gin.LogFormatterParams) string {
				// custom JSON format
				return fmt.Sprintf(`{"level":"%s", "client-ip":"%s", "ts":"%s", "method":"%s", "path":"%s", "proto":"%s", "status":"%d", "latency":"%s", "user-agent":"%s", "error-message":"%s"}`+"\n",
					"info",
					params.ClientIP,
					params.TimeStamp.Format(time.RFC1123),
					params.Method,
					params.Path,
					params.Request.Proto,
					params.StatusCode,
					params.Latency,
					params.Request.UserAgent(),
					params.ErrorMessage,
				)
			},
		}),
		gin.Recovery(),
		// When the origin header is specified, cors middleware will expose the cors functionality and the
		// OPTIONS endpoint may be executed. OPTIONS will return a status code  of 204 no content.
		// If the origin is the same, the cors functionality is skipped and OPTIONS endpoint cannot be
		// successfully called. Executing an OPTIONS request when from the same origin will result
		// in a 403 forbidden response.
		cors.New(cors.Config{
			AllowAllOrigins:  true,
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

	srv.httpServer = &http.Server{
		Addr: configuration.HTTPAddress,
		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout: configuration.HTTPWriteTimeout,
		ReadTimeout:  configuration.HTTPReadTimeout,
		IdleTimeout:  configuration.HTTPIdleTimeout,
		Handler:      srv.router,
	}
	if configuration.HTTPCompressResponses {
		srv.router.Use(gzip.Gzip(gzip.DefaultCompression))
	}
	return srv
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
