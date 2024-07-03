package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	MetricLabelRejected  = "Rejected"
	MetricsLabelVerbGet  = "Get"
	MetricsLabelVerbList = "List"
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
	reg.MustRegister(regServProxyAPIHistogramVec)
	reg.MustRegister(regServWorkspaceHistogramVec)
	return &ProxyMetrics{
		RegServWorkspaceHistogramVec: regServWorkspaceHistogramVec,
		RegServProxyAPIHistogramVec:  regServProxyAPIHistogramVec,
		Reg:                          reg,
	}
}

func newHistogramVec(name, help string, labels ...string) *prometheus.HistogramVec {
	v := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    metricsPrefix + name,
		Help:    help,
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 5, 10},
	}, labels)
	return v
}
