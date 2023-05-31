package server

import (
	"github.com/codeready-toolchain/registration-service/pkg/assets"
	"github.com/codeready-toolchain/registration-service/pkg/auth"
	"github.com/gin-contrib/static"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/controller"
	"github.com/codeready-toolchain/registration-service/pkg/middleware"

	errs "github.com/pkg/errors"
)

// SetupRoutes registers handlers for various URL paths.
func (srv *RegistrationServer) SetupRoutes() error {
	var err error
	_, err = auth.InitializeDefaultTokenParser()
	if err != nil {
		return err
	}

	srv.routesSetup.Do(func() {
		// creating the controllers
		healthCheckCtrl := controller.NewHealthCheck(controller.NewHealthChecker())
		authConfigCtrl := controller.NewAuthConfig()
		analyticsCtrl := controller.NewAnalytics()
		signupCtrl := controller.NewSignup(srv.application)

		// create the auth middleware
		var authMiddleware *middleware.JWTMiddleware
		authMiddleware, err = middleware.NewAuthMiddleware()
		if err != nil {
			err = errs.Wrapf(err, "failed to init auth middleware")
			return
		}

		// unsecured routes
		unsecuredV1 := srv.router.Group("/api/v1")
		unsecuredV1.GET("/health", healthCheckCtrl.GetHandler)
		unsecuredV1.GET("/authconfig", authConfigCtrl.GetHandler)
		unsecuredV1.GET("/segment-write-key", analyticsCtrl.GetDevSpacesSegmentWriteKey) //expose the devspaces segment key

		// secured routes
		securedV1 := srv.router.Group("/api/v1")
		securedV1.Use(authMiddleware.HandlerFunc())
		securedV1.POST("/signup", signupCtrl.PostHandler)
		// requires a ctx body containing the country_code and phone_number
		securedV1.PUT("/signup/verification", signupCtrl.InitVerificationHandler)
		securedV1.GET("/signup", signupCtrl.GetHandler)
		securedV1.GET("/signup/verification/:code", signupCtrl.VerifyPhoneCodeHandler) // TODO: also provide a `POST /signup/verification/phone-code` +deprecate this one + migrate UI?
		securedV1.POST("/signup/verification/activation-code", signupCtrl.VerifyActivationCodeHandler)

		// if we are in testing mode, we also add a secured health route for testing
		if configuration.IsTestingMode() {
			securedV1.GET("/auth_test", healthCheckCtrl.GetHandler)
		}

		// Create the route for static content, served from /
		var staticHandler static.ServeFileSystem
		staticHandler, err = assets.ServeEmbedContent()
		if err != nil {
			err = errs.Wrap(err, "unable to setup route to serve static content")
		}
		srv.router.Use(static.Serve("/", staticHandler))

	})
	return err
}
