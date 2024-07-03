package server

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const RegSvcMetricsPort = 8083

var log = logf.Log.WithName("registration_metrics")

// return a new Gin server exposing the `/metrics` endpoint associated with the given Prometheus registry
func StartMetricsServer(reg *prometheus.Registry, port int) (*http.Server, *gin.Engine) {
	router := gin.Default()
	router.GET("/metrics", gin.WrapH(promhttp.InstrumentMetricHandler(
		reg, promhttp.HandlerFor(reg, promhttp.HandlerOpts{
			DisableCompression: true,
		}),
	)))
	log.Info("Starting the registration-service metrics server...")
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           router.Handler(),
		ReadHeaderTimeout: 2 * time.Second,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			NextProtos: []string{"http/1.1"}, // disable HTTP/2 for now
		},
	}
	go func() {
		// service connections
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error(err, err.Error())
		}
	}()
	return srv, router
}
