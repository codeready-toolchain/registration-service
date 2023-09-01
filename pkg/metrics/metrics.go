package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("registration_metrics")
var Reg *prometheus.Registry

const (
	ResponseMetricLabelApprove = "Approve"
	ResponseMetricLabelReject  = "Reject"
)

// histogram with labels
var (
	// RegServProxyRouteHistogramVec measures the time taken by proxy before forwarding the request
	RegServProxyRouteHistogramVec *prometheus.HistogramVec
	// RegServProxyResponseHistogramVec measures the response time for either response or error from proxy when there is no routing
	RegServProxyResponseHistogramVec *prometheus.HistogramVec
)

// collections
var (
	allHistogramVecs = []*prometheus.HistogramVec{}
)

const metricsPrefix = "sandbox_"

func initMetrics() {
	log.Info("initializing custom metrics")
	RegServProxyRouteHistogramVec = newHistogramVec("proxy_route_time", "time taken by proxy to route ", "routeTo")
	RegServProxyResponseHistogramVec = newHistogramVec("proxy_response_time", "time for response of a request to proxy ", "responseFor")
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
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10},
	}, labels)
	allHistogramVecs = append(allHistogramVecs, v)
	return v
}

// RegisterCustomMetrics registers the custom metrics
func RegisterCustomMetrics() {
	initMetrics()
	Reg = prometheus.NewRegistry()
	// register metrics
	for _, v := range allHistogramVecs {
		if err := Reg.Register(v); err != nil {
			log.Error(err, "failed to register histogramVec", "Histogram Name:", v)
		}
	}
	log.Info("custom metrics registered")
}
