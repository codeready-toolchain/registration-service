package registrationserver

import (
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	errs "github.com/pkg/errors"
)

// RegistrationServer bundles configuration, logging, and HTTP server objects in a single
// location.
type RegistrationServer struct {
	config     *configuration.Registry
	router     *mux.Router
	httpServer *http.Server

	logger      *log.Logger
	routesSetup sync.Once
}

// New creates a new RegistrationServer object with reasonable defaults.
func New(configFilePath string) (*RegistrationServer, error) {
	srv := &RegistrationServer{
		router: mux.NewRouter(),
		logger: log.New(os.Stdout, "", 0),
	}
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
		Handler:      handlers.CombinedLoggingHandler(os.Stdout, srv.router),
	}
	if srv.config.GetHTTPCompressResponses() {
		srv.router.Use(handlers.CompressHandler)
	}
	return srv, nil
}

// Logger returns the app server's log object
func (srv *RegistrationServer) Logger() *log.Logger {
	return srv.logger
}

// Config returns the app server's config object
func (srv *RegistrationServer) Config() *configuration.Registry {
	return srv.config
}

// HTTPServer returns the app server's HTTP server
func (srv *RegistrationServer) HTTPServer() *http.Server {
	return srv.httpServer
}

// Router returns the app server's HTTP router
func (srv *RegistrationServer) Router() *mux.Router {
	return srv.router
}

// GetRegisteredRoutes returns all registered routes formatted with their
// methods, paths, queries and names. It is a good idea to print this
// information on server start to give you an idea of what routes are
// available in the system.
func (srv *RegistrationServer) GetRegisteredRoutes() (string, error) {
	var sb strings.Builder
	err := srv.router.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {

		sb.WriteString("ROUTE: ")
		pathTemplate, err := route.GetPathTemplate()
		if err == nil {
			sb.WriteString("\tPath template: ")
			sb.WriteString(pathTemplate)
		}
		name := route.GetName()
		if name != "" {
			sb.WriteString("\n\tName: ")
			sb.WriteString(name)
		}
		pathRegexp, err := route.GetPathRegexp()
		if err == nil {
			sb.WriteString("\n\tPath regexp: ")
			sb.WriteString(pathRegexp)
		}
		queriesTemplates, err := route.GetQueriesTemplates()
		if err == nil {
			sb.WriteString("\n\tQueries templates: ")
			sb.WriteString(strings.Join(queriesTemplates, ","))
		}
		queriesRegexps, err := route.GetQueriesRegexp()
		if err == nil {
			sb.WriteString("\n\tQueries regexps: ")
			sb.WriteString(strings.Join(queriesRegexps, ","))
		}
		methods, err := route.GetMethods()
		if err == nil {
			sb.WriteString("\n\tMethods: ")
			sb.WriteString(strings.Join(methods, ","))
		}
		sb.WriteString("\n")
		return nil
	})
	return sb.String(), err
}
