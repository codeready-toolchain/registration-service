package server

import (
	"log"
	"net/http"
	"path/filepath"

	"github.com/codeready-toolchain/registration-service/pkg/auth"
	"github.com/codeready-toolchain/registration-service/pkg/controller"
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
		healthCheckCtrl := controller.NewHealthCheck(srv.logger, srv.Config())
		authConfigCtrl := controller.NewAuthConfig(srv.logger, srv.Config())
		signupCtrl := controller.NewSignup(srv.logger, srv.Config())

		// get the auth middleware
		var authMiddleware *auth.JWTMiddleware
		authMiddleware, err = auth.NewAuthMiddleware(srv.logger, srv.config)
	
		// public routes
		publicV1 := srv.router.Group("/api/v1")
		publicV1.GET("/health", healthCheckCtrl.GetHandler)
		publicV1.GET("/authconfig", authConfigCtrl.GetHandler)

		// private routes
		privateV1 := srv.router.Group("/api/v1")
		privateV1.Use(authMiddleware.HandlerFunc())
		privateV1.POST("/signup", signupCtrl.PostHandler)

		// if we are in testing mode, we also add a private health route for testing
		if srv.Config().IsTestingMode() {
			privateV1.GET("/health_private", healthCheckCtrl.GetHandler)
		}

		// Create the route for static content, served from /
		static := StaticHandler{Assets: static.Assets}
		// capturing all non-matching routes, assuming them to be static content
		srv.router.NoRoute(static.ServeHTTP)
	})
	return err
}
