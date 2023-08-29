package metrics

import (
	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	k8smetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
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
	//obs, err := m.GetMetricWithLabelValues("approve request")
	//require.NoError(t, err)
	//metric := &dto.Metric{}

	//assert.Equal(t, float64(3), promtestutil.ToFloat64(m.WithLabelValues("member-2")))
}

func TestRegisterCustomMetrics(t *testing.T) {
	// when
	RegisterCustomMetrics()

	// then
	// verify all metrics were registered successfully
	for _, m := range allHistogramVecs {
		assert.True(t, k8smetrics.Registry.Unregister(m))
	}
}
