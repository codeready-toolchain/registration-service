package metrics

import (
	"github.com/labstack/echo/v4"
	glog "github.com/labstack/gommon/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("registration_metrics")

const (
	MetricLabelRejected  = "Rejected"
	MetricsLabelVerbGet  = "Get"
	MetricsLabelVerbList = "List"
	MetricsPort          = "8082"
)

type ProxyMetrics struct {
	// RegServProxyAPIHistogramVec measures the time taken by proxy before forwarding the request
	RegServProxyAPIHistogramVec *prometheus.HistogramVec
	// RegServWorkspaceHistogramVec measures the response time for either response or error from proxy when there is no routing
	RegServWorkspaceHistogramVec *prometheus.HistogramVec
	Reg                          *prometheus.Registry
}

const metricsPrefix = "sandbox_"

func NewProxyMetrics(reg *prometheus.Registry) *ProxyMetrics {
	regServProxyAPIHistogramVec := newHistogramVec("proxy_api_http_request_time", "time taken by proxy to route to a target cluster", "status_code", "route_to")
	regServWorkspaceHistogramVec := newHistogramVec("proxy_workspace_http_request_time", "time for response of a request to proxy ", "status_code", "kube_verb")
	metrics := &ProxyMetrics{
		RegServWorkspaceHistogramVec: regServWorkspaceHistogramVec,
		RegServProxyAPIHistogramVec:  regServProxyAPIHistogramVec,
		Reg:                          reg,
	}
	metrics.Reg.MustRegister(metrics.RegServProxyAPIHistogramVec)
	metrics.Reg.MustRegister(metrics.RegServWorkspaceHistogramVec)
	return metrics
}

func newHistogramVec(name, help string, labels ...string) *prometheus.HistogramVec {
	v := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    metricsPrefix + name,
		Help:    help,
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 5, 10},
	}, labels)
	return v
}

func (p *ProxyMetrics) StartMetricsServer() *http.Server {
	// start server
	srv := echo.New()
	srv.Logger.SetLevel(glog.INFO)
	srv.GET("/metrics", echo.WrapHandler(promhttp.HandlerFor(p.Reg, promhttp.HandlerOpts{DisableCompression: true, Registry: p.Reg})))

	log.Info("Starting the Registration-Service Metrics server...")
	// listen concurrently to allow for graceful shutdown
	go func() {
		if err := srv.Start(":" + MetricsPort); err != http.ErrServerClosed {
			log.Error(err, err.Error())
		}
	}()

	return srv.Server
}
