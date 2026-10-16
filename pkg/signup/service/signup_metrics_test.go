package service

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterSignupMetrics(t *testing.T) {
	// given
	reg := prometheus.NewRegistry()
	UserSignupsGatingOnlyTotal.Reset()

	// when
	require.NotPanics(t, func() {
		RegisterSignupMetrics(reg)
	})

	// then — observe once so Gather includes the vector
	UserSignupsGatingOnlyTotal.WithLabelValues(GatingOnlyStateVerified).Inc()

	gathered, err := reg.Gather()
	require.NoError(t, err)
	names := make(map[string]bool, len(gathered))
	for _, m := range gathered {
		names[m.GetName()] = true
	}
	assert.True(t, names["user_signups_gating_only_total"])
}
