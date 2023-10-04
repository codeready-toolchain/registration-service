package metrics

import (
	"github.com/labstack/echo/v4"
	glog "github.com/labstack/gommon/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

var log = logf.Log.WithName("registration_metrics")
var Reg *prometheus.Registry

const (
	MetricLabelRejected  = "Rejected"
	MetricsLabelVerbGet  = "Get"
	MetricsLabelVerbList = "List"
	MetricsPort          = "8082"
)

// histogram with labels
var (
	// RegServProxyAPIHistogramVec measures the time taken by proxy before forwarding the request
	RegServProxyAPIHistogramVec *prometheus.HistogramVec
	// RegServWorkspaceHistogramVec measures the response time for either response or error from proxy when there is no routing
	RegServWorkspaceHistogramVec *prometheus.HistogramVec
)

// collections
var (
	allHistogramVecs = []*prometheus.HistogramVec{}
)

const metricsPrefix = "sandbox_"

func init() {
	initMetrics()
}
func initMetrics() {
	log.Info("initializing custom metrics")
	RegServProxyAPIHistogramVec = newHistogramVec("proxy_api_http_request_time", "time taken by proxy to route to a target cluster", "status_code", "route_to")
	RegServWorkspaceHistogramVec = newHistogramVec("proxy_workspace_http_request_time", "time for response of a request to proxy ", "status_code", "kube_verb")
	log.Info("custom metrics initialized")
}

// Reset resets all metrics. For testing purpose only!
func Reset() {
	log.Info("resetting custom metrics")
	initMetrics()
}

func newHistogramVec(name, help string, labels ...string) *prometheus.HistogramVec {
	v := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    metricsPrefix + name,
		Help:    help,
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 5, 10},
	}, labels)
	allHistogramVecs = append(allHistogramVecs, v)
	return v
}

// RegisterCustomMetrics registers the custom metrics
func RegisterCustomMetrics() {
	Reg = prometheus.NewRegistry()
	// register metrics
	for _, v := range allHistogramVecs {
		Reg.MustRegister(v)
	}
	log.Info("custom metrics registered")
}

type Metrics struct{}

func NewMetrics() *Metrics {
	return &Metrics{}
}

//nolint:unparam
func (m *Metrics) PrometheusHandler(ctx echo.Context) error {
	h := promhttp.HandlerFor(Reg, promhttp.HandlerOpts{DisableCompression: true, Registry: Reg})
	h.ServeHTTP(ctx.Response().Writer, ctx.Request())
	return nil
}

func (m *Metrics) StartMetricsServer() *http.Server {
	// start server
	router := echo.New()
	router.Logger.SetLevel(glog.INFO)
	router.GET("/metrics", m.PrometheusHandler)

	log.Info("Starting the Registration-Service Metrics server...")
	srv := &http.Server{Addr: ":" + MetricsPort, Handler: router, ReadHeaderTimeout: 2 * time.Second}
	// listen concurrently to allow for graceful shutdown
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Error(err, err.Error())
		}
	}()

	return srv
}
