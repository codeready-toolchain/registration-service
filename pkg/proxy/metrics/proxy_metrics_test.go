package metrics

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"
	clientmodel "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.Len(t, g, 1)
	require.Equal(t, "sandbox_test_histogram_vec", g[0].GetName())
	require.Equal(t, "test histogram description", g[0].GetHelp())
	require.Len(t, g[0].GetMetric(), 4)

	// let's confirm the count of each label combination
	require.Len(t, g[0].GetMetric()[0].GetLabel(), 2)
	compareLabelPairValues(t, getSuccess, g[0].GetMetric()[0].GetLabel())
	require.Equal(t, uint64(1), g[0].GetMetric()[0].GetHistogram().GetSampleCount())

	require.Len(t, g[0].GetMetric()[1].GetLabel(), 2)
	compareLabelPairValues(t, getFailure, g[0].GetMetric()[1].GetLabel())
	require.Equal(t, uint64(1), g[0].GetMetric()[1].GetHistogram().GetSampleCount())

	require.Len(t, g[0].GetMetric()[2].GetLabel(), 2)
	compareLabelPairValues(t, listSuccess, g[0].GetMetric()[2].GetLabel())
	require.Equal(t, uint64(1), g[0].GetMetric()[2].GetHistogram().GetSampleCount())

	require.Len(t, g[0].GetMetric()[3].GetLabel(), 2)
	compareLabelPairValues(t, listFailure, g[0].GetMetric()[3].GetLabel())
	require.Equal(t, uint64(2), g[0].GetMetric()[3].GetHistogram().GetSampleCount())

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
