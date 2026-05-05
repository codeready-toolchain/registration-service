package server

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/application"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// requestLog defines the structure with which the requests will be logged by
// the logging middelware of Echo.
type requestLog struct {
	LogLevel     string        `json:"level"`
	ClientIP     string        `json:"client_ip"`
	Timestamp    string        `json:"timestamp"`
	HTTPMethod   string        `json:"method"`
	HTTPPath     string        `json:"path"`
	Protocol     string        `json:"proto"`
	HTTPStatus   int           `json:"status"`
	Latency      time.Duration `json:"latency"`
	UserAgent    string        `json:"user-agent"`
	ErrorMessage error         `json:"error-message"`
}

type ServerOption = func(server *RegistrationServer) // nolint:revive

// RegistrationServer bundles configuration, and HTTP server objects in a single
// location.
type RegistrationServer struct {
	router      *echo.Echo
	httpServer  *http.Server
	routesSetup sync.Once
	application application.Application
}

// New creates a new RegistrationServer object with reasonable defaults.
func New(application application.Application) *RegistrationServer {
	router := echo.New()
	router.HideBanner = true
	router.HidePort = true

	router.Use(
		middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
			Skipper: func(c echo.Context) bool {
				return c.Path() == "/api/v1/health"
			},
			LogMethod:    true,
			LogURI:       true,
			LogStatus:    true,
			LogLatency:   true,
			LogRemoteIP:  true,
			LogProtocol:  true,
			LogUserAgent: true,
			LogError:     true,
			LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
				payload := requestLog{
					LogLevel:     "info",
					ClientIP:     v.RemoteIP,
					Timestamp:    time.Now().Format(time.RFC3339),
					HTTPMethod:   v.Method,
					HTTPPath:     v.URI,
					Protocol:     v.Protocol,
					HTTPStatus:   v.Status,
					Latency:      v.Latency,
					UserAgent:    v.UserAgent,
					ErrorMessage: v.Error,
				}

				return json.NewEncoder(c.Logger().Output()).Encode(payload)
			},
		}),
		middleware.Recover(),
		middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins:     []string{"*"},
			AllowMethods:     []string{"PUT", "PATCH", "POST", "GET", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Content-Length", "Content-Type", "Authorization", "Accept", "Recaptcha-Token"},
			ExposeHeaders:    []string{"Content-Length", "Authorization"},
			AllowCredentials: true,
		}),
	)

	srv := &RegistrationServer{
		router:      router,
		application: application,
	}

	srv.httpServer = &http.Server{
		Addr:         configuration.HTTPAddress,
		WriteTimeout: configuration.HTTPWriteTimeout,
		ReadTimeout:  configuration.HTTPReadTimeout,
		IdleTimeout:  configuration.HTTPIdleTimeout,
		Handler:      srv.router,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			NextProtos: []string{"http/1.1"},
		},
	}
	if configuration.HTTPCompressResponses {
		srv.router.Use(middleware.Gzip())
	}
	return srv
}

// HTTPServer returns the app server's HTTP server.
func (srv *RegistrationServer) HTTPServer() *http.Server {
	return srv.httpServer
}

// Engine returns the app server's Echo router.
func (srv *RegistrationServer) Engine() *echo.Echo {
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
