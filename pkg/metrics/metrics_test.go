package metrics

import (
	"bytes"
	"github.com/prometheus/client_golang/prometheus"
	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"
	clientmodel "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestHistogramVec(t *testing.T) {
	// given
	reg := prometheus.NewRegistry()
	m := newHistogramVec("test_histogram_vec", "test histogram description", "status_code", "kube_verb")
	getSuccess, getFailure, listSuccess, listFailure := getExpectedLabelPairs()
	reg.MustRegister(m)

	// when
	m.WithLabelValues("200", "get").Observe((5 * time.Second).Seconds())
	m.WithLabelValues("404", "get").Observe((3 * time.Second).Seconds())
	m.WithLabelValues("200", "list").Observe((2 * time.Second).Seconds())
	m.WithLabelValues("500", "list").Observe((3 * time.Second).Seconds())
	m.WithLabelValues("500", "list").Observe((1 * time.Millisecond).Seconds())

	//then
	assert.Equal(t, 4, promtestutil.CollectAndCount(m, "sandbox_test_histogram_vec"))
	err := promtestutil.CollectAndCompare(m, strings.NewReader(expectedResponseMetadata+expectedResponse), "sandbox_test_histogram_vec")
	require.NoError(t, err)

	g, er := reg.Gather()
	require.NoError(t, er)
	require.Equal(t, 1, len(g))
	require.Equal(t, "sandbox_test_histogram_vec", g[0].GetName())
	require.Equal(t, "test histogram description", g[0].GetHelp())
	require.Equal(t, 4, len(g[0].Metric))

	// let's confirm the count of each label combination
	require.Equal(t, 2, len(g[0].Metric[0].Label))
	compareLabelPairValues(t, getSuccess, g[0].GetMetric()[0].GetLabel())
	require.Equal(t, uint64(1), *g[0].GetMetric()[0].Histogram.SampleCount)

	require.Equal(t, 2, len(g[0].Metric[1].Label))
	compareLabelPairValues(t, getFailure, g[0].GetMetric()[1].GetLabel())
	require.Equal(t, uint64(1), *g[0].Metric[1].Histogram.SampleCount)

	require.Equal(t, 2, len(g[0].Metric[2].Label))
	compareLabelPairValues(t, listSuccess, g[0].GetMetric()[2].GetLabel())
	require.Equal(t, uint64(1), *g[0].Metric[2].Histogram.SampleCount)

	require.Equal(t, 2, len(g[0].Metric[3].Label))
	compareLabelPairValues(t, listFailure, g[0].GetMetric()[3].GetLabel())
	require.Equal(t, uint64(2), *g[0].Metric[3].Histogram.SampleCount)

}

func TestMetricsServer(t *testing.T) {
	reg := prometheus.NewRegistry()
	testMetrics := NewProxyMetrics(reg)
	server := testMetrics.StartMetricsServer()
	require.NotNil(t, server)
	// Wait up to N seconds for the Metrics server to start
	ready := false
	sec := 10
	for i := 0; i < sec; i++ {
		req, err := http.NewRequest("GET", "http://localhost:8082/metrics", nil)
		require.NoError(t, err)
		require.NotNil(t, req)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			// The server may be running but still not fully ready to accept requests
			time.Sleep(time.Second)
			continue
		}
		// Server is up and running!
		ready = true
		break
	}
	require.True(t, ready, "Metrics Server is not ready after %d seconds", sec)
	defer func() {
		_ = server.Close()
	}()

	req, err := http.NewRequest("GET", "http://localhost:8082/metrics", nil)
	require.NoError(t, err)
	require.NotNil(t, req)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "text/plain; version=0.0.4; charset=utf-8", resp.Header.Get("Content-Type"))
	// compare the body of the response as well
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, expectedServerBlankResponse, buf.String())

}

var expectedResponseMetadata = `
		# HELP sandbox_test_histogram_vec test histogram description
		# TYPE sandbox_test_histogram_vec histogram`
var expectedResponse = `
		sandbox_test_histogram_vec_bucket{kube_verb="get",status_code="200",le="0.05"} 0
		sandbox_test_histogram_vec_bucket{kube_verb="get",status_code="200",le="0.1"} 0
		sandbox_test_histogram_vec_bucket{kube_verb="get",status_code="200",le="0.25"} 0
		sandbox_test_histogram_vec_bucket{kube_verb="get",status_code="200",le="0.5"} 0
		sandbox_test_histogram_vec_bucket{kube_verb="get",status_code="200",le="1"} 0
		sandbox_test_histogram_vec_bucket{kube_verb="get",status_code="200",le="5"} 1
		sandbox_test_histogram_vec_bucket{kube_verb="get",status_code="200",le="10"} 1
		sandbox_test_histogram_vec_bucket{kube_verb="get",status_code="200",le="+Inf"} 1
		sandbox_test_histogram_vec_sum{kube_verb="get",status_code="200"} 5
		sandbox_test_histogram_vec_count{kube_verb="get",status_code="200"} 1
		sandbox_test_histogram_vec_bucket{kube_verb="get",status_code="404",le="0.05"} 0
		sandbox_test_histogram_vec_bucket{kube_verb="get",status_code="404",le="0.1"} 0
		sandbox_test_histogram_vec_bucket{kube_verb="get",status_code="404",le="0.25"} 0
		sandbox_test_histogram_vec_bucket{kube_verb="get",status_code="404",le="0.5"} 0
		sandbox_test_histogram_vec_bucket{kube_verb="get",status_code="404",le="1"} 0
		sandbox_test_histogram_vec_bucket{kube_verb="get",status_code="404",le="5"} 1
		sandbox_test_histogram_vec_bucket{kube_verb="get",status_code="404",le="10"} 1
		sandbox_test_histogram_vec_bucket{kube_verb="get",status_code="404",le="+Inf"} 1
		sandbox_test_histogram_vec_sum{kube_verb="get",status_code="404"} 3
		sandbox_test_histogram_vec_count{kube_verb="get",status_code="404"} 1
		sandbox_test_histogram_vec_bucket{kube_verb="list",status_code="200",le="0.05"} 0
		sandbox_test_histogram_vec_bucket{kube_verb="list",status_code="200",le="0.1"} 0
		sandbox_test_histogram_vec_bucket{kube_verb="list",status_code="200",le="0.25"} 0
		sandbox_test_histogram_vec_bucket{kube_verb="list",status_code="200",le="0.5"} 0
		sandbox_test_histogram_vec_bucket{kube_verb="list",status_code="200",le="1"} 0
		sandbox_test_histogram_vec_bucket{kube_verb="list",status_code="200",le="5"} 1
		sandbox_test_histogram_vec_bucket{kube_verb="list",status_code="200",le="10"} 1
		sandbox_test_histogram_vec_bucket{kube_verb="list",status_code="200",le="+Inf"} 1
		sandbox_test_histogram_vec_sum{kube_verb="list",status_code="200"} 2
		sandbox_test_histogram_vec_count{kube_verb="list",status_code="200"} 1
		sandbox_test_histogram_vec_bucket{kube_verb="list",status_code="500",le="0.05"} 1
		sandbox_test_histogram_vec_bucket{kube_verb="list",status_code="500",le="0.1"} 1
		sandbox_test_histogram_vec_bucket{kube_verb="list",status_code="500",le="0.25"} 1
		sandbox_test_histogram_vec_bucket{kube_verb="list",status_code="500",le="0.5"} 1
		sandbox_test_histogram_vec_bucket{kube_verb="list",status_code="500",le="1"} 1
		sandbox_test_histogram_vec_bucket{kube_verb="list",status_code="500",le="5"} 2
		sandbox_test_histogram_vec_bucket{kube_verb="list",status_code="500",le="10"} 2
		sandbox_test_histogram_vec_bucket{kube_verb="list",status_code="500",le="+Inf"} 2
		sandbox_test_histogram_vec_sum{kube_verb="list",status_code="500"} 3.001
		sandbox_test_histogram_vec_count{kube_verb="list",status_code="500"} 2
		`
var expectedServerBlankResponse = `# HELP promhttp_metric_handler_errors_total Total number of internal errors encountered by the promhttp metric handler.
# TYPE promhttp_metric_handler_errors_total counter
promhttp_metric_handler_errors_total{cause="encoding"} 0
promhttp_metric_handler_errors_total{cause="gathering"} 0
`

func compareLabelPairValues(t *testing.T, expected []clientmodel.LabelPair, labelPairs []*clientmodel.LabelPair) {
	for i := range labelPairs {
		require.Equal(t, expected[i].GetName(), labelPairs[i].GetName())
		require.Equal(t, expected[i].GetValue(), labelPairs[i].GetValue())
	}
}

func createLabelPairs(name, value string) clientmodel.LabelPair {
	return clientmodel.LabelPair{
		Name:  &name,
		Value: &value,
	}
}

func getExpectedLabelPairs() ([]clientmodel.LabelPair, []clientmodel.LabelPair, []clientmodel.LabelPair, []clientmodel.LabelPair) {

	// labelPairs are ordered alphabetically on name when gathered.
	getSuccess := []clientmodel.LabelPair{
		createLabelPairs("kube_verb", "get"),
		createLabelPairs("status_code", "200"),
	}
	getFailure := []clientmodel.LabelPair{
		createLabelPairs("kube_verb", "get"),
		createLabelPairs("status_code", "404"),
	}
	listSuccess := []clientmodel.LabelPair{
		createLabelPairs("kube_verb", "list"),
		createLabelPairs("status_code", "200"),
	}
	listFailure := []clientmodel.LabelPair{
		createLabelPairs("kube_verb", "list"),
		createLabelPairs("status_code", "500"),
	}
	return getSuccess, getFailure, listSuccess, listFailure
}
