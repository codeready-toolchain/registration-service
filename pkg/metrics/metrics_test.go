package metrics

import (
	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestHistogramVec(t *testing.T) {
	// given
	m := newHistogramVec("test_histogram_vec", "test histogram description", "responseFor")

	// when
	m.WithLabelValues("approve request").Observe(5)
	m.WithLabelValues("reject request").Observe(3)

	// then
	assert.Equal(t, 2, promtestutil.CollectAndCount(m, "sandbox_test_histogram_vec"))

	err := promtestutil.CollectAndCompare(m, strings.NewReader(expectedResponseMetadata+expectedResponse), "sandbox_test_histogram_vec")
	require.NoError(t, err)
}

func TestRegisterCustomMetrics(t *testing.T) {
	// when
	RegisterCustomMetrics()

	// then
	// verify all metrics were registered successfully
	for _, m := range allHistogramVecs {
		assert.True(t, Reg.Unregister(m))
	}
}

var expectedResponseMetadata = `
		# HELP sandbox_test_histogram_vec test histogram description
		# TYPE sandbox_test_histogram_vec histogram`
var expectedResponse = `
		sandbox_test_histogram_vec_bucket{responseFor="approve request",le="0.05"} 0
		sandbox_test_histogram_vec_bucket{responseFor="approve request",le="0.1"} 0
		sandbox_test_histogram_vec_bucket{responseFor="approve request",le="0.25"} 0
		sandbox_test_histogram_vec_bucket{responseFor="approve request",le="0.5"} 0
		sandbox_test_histogram_vec_bucket{responseFor="approve request",le="1"} 0
		sandbox_test_histogram_vec_bucket{responseFor="approve request",le="2"} 0
		sandbox_test_histogram_vec_bucket{responseFor="approve request",le="5"} 1
		sandbox_test_histogram_vec_bucket{responseFor="approve request",le="10"} 1
		sandbox_test_histogram_vec_bucket{responseFor="approve request",le="+Inf"} 1
		sandbox_test_histogram_vec_sum{responseFor="approve request"} 5
		sandbox_test_histogram_vec_count{responseFor="approve request"} 1
		sandbox_test_histogram_vec_bucket{responseFor="reject request",le="0.05"} 0
		sandbox_test_histogram_vec_bucket{responseFor="reject request",le="0.1"} 0
		sandbox_test_histogram_vec_bucket{responseFor="reject request",le="0.25"} 0
		sandbox_test_histogram_vec_bucket{responseFor="reject request",le="0.5"} 0
		sandbox_test_histogram_vec_bucket{responseFor="reject request",le="1"} 0
		sandbox_test_histogram_vec_bucket{responseFor="reject request",le="2"} 0
		sandbox_test_histogram_vec_bucket{responseFor="reject request",le="5"} 1
		sandbox_test_histogram_vec_bucket{responseFor="reject request",le="10"} 1
		sandbox_test_histogram_vec_bucket{responseFor="reject request",le="+Inf"} 1
		sandbox_test_histogram_vec_sum{responseFor="reject request"} 3
		sandbox_test_histogram_vec_count{responseFor="reject request"} 1
		`
