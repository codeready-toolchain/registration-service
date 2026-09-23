package service

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterPhoneLookupMetrics(t *testing.T) {
	// given
	reg := prometheus.NewRegistry()
	PhoneLookupTotal.Reset()
	PhoneLookupErrorsTotal.Reset()

	// when
	require.NotPanics(t, func() {
		RegisterPhoneLookupMetrics(reg)
	})

	// then — observe once so Gather includes the vectors
	PhoneLookupTotal.WithLabelValues(PhoneLookupResultAllowed, "low", "false").Inc()
	PhoneLookupErrorsTotal.WithLabelValues("500", "true").Inc()

	gathered, err := reg.Gather()
	require.NoError(t, err)
	names := make(map[string]bool, len(gathered))
	for _, m := range gathered {
		names[m.GetName()] = true
	}
	assert.True(t, names["sandbox_signup_phone_lookup_total"])
	assert.True(t, names["sandbox_signup_phone_lookup_errors_total"])
}

func TestPhoneLookupTotalIncrements(t *testing.T) {
	// given
	PhoneLookupTotal.Reset()

	// when
	PhoneLookupTotal.WithLabelValues(PhoneLookupResultAllowed, "low", "false").Inc()
	PhoneLookupTotal.WithLabelValues(PhoneLookupResultBlocked, "high", "true").Inc()
	PhoneLookupTotal.WithLabelValues(PhoneLookupResultBlocked, "high", "true").Inc()

	// then
	assert.InDelta(t, float64(1), promtestutil.ToFloat64(PhoneLookupTotal.WithLabelValues(PhoneLookupResultAllowed, "low", "false")), 0.01)
	assert.InDelta(t, float64(2), promtestutil.ToFloat64(PhoneLookupTotal.WithLabelValues(PhoneLookupResultBlocked, "high", "true")), 0.01)
	assert.Equal(t, 2, promtestutil.CollectAndCount(PhoneLookupTotal))
}

func TestPhoneLookupErrorsTotalIncrements(t *testing.T) {
	// given
	PhoneLookupErrorsTotal.Reset()

	// when
	PhoneLookupErrorsTotal.WithLabelValues("500", "false").Inc()
	PhoneLookupErrorsTotal.WithLabelValues("500", "false").Inc()
	PhoneLookupErrorsTotal.WithLabelValues("503", "true").Inc()

	// then
	assert.InDelta(t, float64(2), promtestutil.ToFloat64(PhoneLookupErrorsTotal.WithLabelValues("500", "false")), 0.01)
	assert.InDelta(t, float64(1), promtestutil.ToFloat64(PhoneLookupErrorsTotal.WithLabelValues("503", "true")), 0.01)
	assert.Equal(t, 2, promtestutil.CollectAndCount(PhoneLookupErrorsTotal))
}
