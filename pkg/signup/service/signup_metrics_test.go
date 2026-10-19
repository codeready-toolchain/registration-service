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

	// then — every known state is already exported at 0
	gathered, err := reg.Gather()
	require.NoError(t, err)
	var states map[string]float64
	for _, family := range gathered {
		if family.GetName() != "user_signups_gating_only_total" {
			continue
		}
		states = make(map[string]float64, len(family.GetMetric()))
		for _, metric := range family.GetMetric() {
			for _, label := range metric.GetLabel() {
				if label.GetName() == "state" {
					states[label.GetValue()] = metric.GetCounter().GetValue()
				}
			}
		}
	}
	assert.Equal(t, map[string]float64{
		GatingOnlyStateRejected:      0,
		GatingOnlyStateBanned:        0,
		GatingOnlyStatePhoneRequired: 0,
		GatingOnlyStatePhoneVerified: 0,
		GatingOnlyStateVerified:      0,
	}, states)
}
