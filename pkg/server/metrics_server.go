package server

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const RegSvcMetricsPort = 8083

var log = logf.Log.WithName("registration_metrics")

func StartMetricsServer(reg *prometheus.Registry, port int) (*http.Server, *echo.Echo) {
	router := echo.New()
	router.HideBanner = true
	router.HidePort = true
	router.GET("/metrics", echo.WrapHandler(promhttp.InstrumentMetricHandler(
		reg, promhttp.HandlerFor(reg, promhttp.HandlerOpts{
			DisableCompression: true,
		}),
	)))

	log.Info("Starting the registration-service metrics server...")
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           router,
		ReadHeaderTimeout: 2 * time.Second,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			NextProtos: []string{"http/1.1"},
		},
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error(err, err.Error())
		}
	}()
	return srv, router
}
