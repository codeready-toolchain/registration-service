package server

import (
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/assets"
	"github.com/codeready-toolchain/registration-service/pkg/auth"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	rcontext "github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/controller"
	"github.com/codeready-toolchain/registration-service/pkg/middleware"
	"github.com/codeready-toolchain/registration-service/pkg/namespaced"
	"github.com/gin-gonic/gin"

	"github.com/gin-contrib/static"
	errs "github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

// SetupRoutes registers handlers for various URL paths.
// proxyPort is the API Proxy Server port to be used to setup a route for the health checker for the proxy.
func (srv *RegistrationServer) SetupRoutes(proxyPort string, reg *prometheus.Registry, nsClient namespaced.Client) error {
	var err error
	_, err = auth.InitializeDefaultTokenParser()
	if err != nil {
		return err
	}

	inFlightGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "sandbox_promhttp_client_in_flight_requests",
		Help: "A gauge of in-flight requests for the wrapped client.",
	})

	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sandbox_promhttp_client_api_requests_total",
			Help: "A counter for requests from the wrapped client.",
		},
		[]string{"code", "method", "path"},
	)

	// histVec has no labels, making it a zero-dimensional ObserverVec.
	histVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sandbox_promhttp_request_duration_seconds",
			Help:    "A histogram of request latencies.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"code", "method", "path"},
	)

	// Register all of the metrics in the standard registry.
	reg.MustRegister(counter, histVec, inFlightGauge)

	srv.routesSetup.Do(func() {
		// creating the controllers
		healthCheckCtrl := controller.NewHealthCheck(controller.NewHealthChecker(proxyPort))
		authConfigCtrl := controller.NewAuthConfig()
		analyticsCtrl := controller.NewAnalytics()
		signupCtrl := controller.NewSignup(srv.application)
		usernamesCtrl := controller.NewUsernames(nsClient)

		// unsecured routes
		unsecuredV1 := srv.router.Group("/api/v1")
		unsecuredV1.Use(
			middleware.InstrumentRoundTripperInFlight(inFlightGauge),
			middleware.InstrumentRoundTripperCounter(counter),
			middleware.InstrumentRoundTripperDuration(histVec))
		unsecuredV1.GET("/health", healthCheckCtrl.GetHandler) // TODO: move to root (`/`)?
		unsecuredV1.GET("/authconfig", authConfigCtrl.GetHandler)
		unsecuredV1.GET("/segment-write-key", analyticsCtrl.GetDevSpacesSegmentWriteKey) //expose the devspaces segment key

		// create the auth middleware
		var authMiddleware *middleware.JWTMiddleware
		authMiddleware, err = middleware.NewAuthMiddleware()
		if err != nil {
			err = errs.Wrapf(err, "failed to init auth middleware")
			return
		}
		receivedTimeMw := func(ctx *gin.Context) {
			ctx.Set(rcontext.RequestReceivedTime, time.Now())
		}
		// secured routes
		securedV1 := srv.router.Group("/api/v1")
		securedV1.Use(
			middleware.InstrumentRoundTripperInFlight(inFlightGauge),
			middleware.InstrumentRoundTripperCounter(counter),
			middleware.InstrumentRoundTripperDuration(histVec),
			authMiddleware.HandlerFunc(),
			receivedTimeMw)
		securedV1.POST("/signup", signupCtrl.PostHandler)
		// requires a ctx body containing the country_code and phone_number
		securedV1.PUT("/signup/verification", signupCtrl.InitVerificationHandler)
		securedV1.GET("/signup", signupCtrl.GetHandler)
		securedV1.GET("/signup/verification/:code", signupCtrl.VerifyPhoneCodeHandler) // TODO: also provide a `POST /signup/verification/phone-code` +deprecate this one + migrate UI?
		securedV1.POST("/signup/verification/activation-code", signupCtrl.VerifyActivationCodeHandler)
		securedV1.GET("/usernames/:username", usernamesCtrl.GetHandler)

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
