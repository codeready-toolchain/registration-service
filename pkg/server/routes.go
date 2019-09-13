package server

import (
	"log"
	"net/http"
	"path/filepath"

	"github.com/codeready-toolchain/registration-service/pkg/health"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/registration-service/pkg/static"
	"github.com/gin-gonic/gin"
)

// StaticHandler implements the http.Handler interface, so we can use it
// to respond to HTTP requests. The path to the static directory and
// path to the index file within that static directory are used to
// serve the SPA in the given static directory.
type StaticHandler struct {
	Assets http.FileSystem
}

// ServeHTTP inspects the URL path to locate a file within the static dir
// on the SPA handler. If a file is found, it will be served. If not, the
// file located at the index path on the SPA handler will be served. This
// is suitable behavior for serving an SPA (single page application).
func (h StaticHandler) ServeHTTP(ctx *gin.Context) {
	// Get the absolute path to prevent directory traversal
	path, err := filepath.Abs(ctx.Request.URL.Path)
	if err != nil {
		// No absolute path, respond with a 400 bad request and stop
		http.Error(ctx.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if the file exists in the assets.
	_, err = h.Assets.Open(path)
	if err != nil {
		// File does not exist, redirect to index.
		log.Printf("File %s does not exist.", path)
		http.Redirect(ctx.Writer, ctx.Request, "/index.html", http.StatusSeeOther)
		return
	}

	// Otherwise, use http.FileServer to serve the static dir.
	http.FileServer(h.Assets).ServeHTTP(ctx.Writer, ctx.Request)
}

// SetupRoutes registers handlers for various URL paths. You can call this
// function more than once but only the first call will have an effect.
func (srv *RegistrationServer) SetupRoutes() error {
	var err error
	srv.routesSetup.Do(func() {

		// /status is something you should always have in any of your services,
		// please leave it as is.
		healthService := health.NewHealthCheckService(srv.logger, srv.Config())
		signupService, err := signup.NewSignupService(srv.logger, srv.Config())
		if err != nil {
			// error creating signup service, bail out.
			log.Fatal(err)
		}
		
		v1 := srv.router.Group("/api/v1")
		{
			v1.GET("/health", healthService.GetHealthCheckHandler)
			v1.POST("/signup", signupService.PostSignupHandler)
		}

		// Create the route for static content, served from /
		static := StaticHandler{Assets: static.Assets}
		// capturing all non-matching routes, assuming them to be static content
		srv.router.NoRoute(static.ServeHTTP)
	})
	return err
}
