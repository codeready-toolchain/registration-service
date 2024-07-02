package proxy

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	glog "github.com/labstack/gommon/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const ProxyMetricsPort = 8082

// StartMetricsServer start server with a single `/metrics` endpoint to server the Prometheus metrics
// Uses echo web framework
func StartMetricsServer(reg *prometheus.Registry, port int) *http.Server {
	log := logf.Log.WithName("proxy_metrics")
	srv := echo.New()
	srv.Logger.SetLevel(glog.INFO)
	srv.GET("/metrics", echo.WrapHandler(promhttp.HandlerFor(reg, promhttp.HandlerOpts{DisableCompression: true, Registry: reg})))
	srv.DisableHTTP2 = true // disable HTTP/2 for now

	log.Info("Starting the proxy metrics server...")
	// listen concurrently to allow for graceful shutdown
	go func() {
		if err := srv.Start(fmt.Sprintf(":%d", port)); !errors.Is(err, http.ErrServerClosed) {
			log.Error(err, err.Error())
		}
	}()

	return srv.Server
}
