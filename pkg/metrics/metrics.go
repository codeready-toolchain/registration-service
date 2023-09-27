package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("registration_metrics")
var Reg *prometheus.Registry

const (
	MetricLabelRejected  = "Rejected"
	MetricsLabelVerbGet  = "Get"
	MetricsLabelVerbList = "List"
)

// histogram with labels
var (
	// RegServProxyApiHistogramVec measures the time taken by proxy before forwarding the request
	RegServProxyApiHistogramVec *prometheus.HistogramVec
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
	RegServProxyApiHistogramVec = newHistogramVec("proxy_api_http_request_time", "time taken by proxy to route to a target cluster", "status_code", "route_to")
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
