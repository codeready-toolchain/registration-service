package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	k8smetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var log = logf.Log.WithName("toolchain_metrics")

// histogram with labels
var (
	// RegistrationServiceProxyRoute measures the time to route a message from proxy
	RegistrationServiceProxyRoute    *prometheus.HistogramVec
	RegistrationServiceProxyResponse *prometheus.HistogramVec
)

// collections
var (
	allHistogramVecs = []*prometheus.HistogramVec{}
)

func init() {
	initMetrics()
}

const metricsPrefix = "sandbox_"

func initMetrics() {
	log.Info("initializing custom metrics")
	RegistrationServiceProxyRoute = newHistogramVec("proxy_route_time", "time taken by proxy to route ", "routeTo")
	RegistrationServiceProxyResponse = newHistogramVec("proxy_response_time", "time for response of a request to proxy ", "responseFor")
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
	// register metrics
	for _, v := range allHistogramVecs {
		k8smetrics.Registry.MustRegister(v)
	}
	log.Info("custom metrics registered")
}
