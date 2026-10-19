package service

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	GatingOnlyStateRejected      = "rejected"
	GatingOnlyStateBanned        = "banned"
	GatingOnlyStatePhoneRequired = "phone_required"
	GatingOnlyStatePhoneVerified = "phone_verified"
	GatingOnlyStateVerified      = "verified"
)

var (
	// UserSignupsGatingOnlyTotal counts a UserSignup when used for gating-only requests,
	// labeled by the state observed on that request.
	UserSignupsGatingOnlyTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "user_signups_gating_only_total",
			Help: "Total UserSignups newly marked as gating-only, by state (rejected, banned, or verified).",
		},
		[]string{"state"},
	)
)

// RegisterSignupMetrics registers signup metrics with the given registry.
// Each known state is created at 0 so the first increment is visible to rate and increase.
func RegisterSignupMetrics(reg *prometheus.Registry) {
	reg.MustRegister(UserSignupsGatingOnlyTotal)
	for _, state := range []string{
		GatingOnlyStateRejected,
		GatingOnlyStateBanned,
		GatingOnlyStatePhoneRequired,
		GatingOnlyStatePhoneVerified,
		GatingOnlyStateVerified,
	} {
		UserSignupsGatingOnlyTotal.WithLabelValues(state)
	}
}

func IncGatingOnly(state string) {
	UserSignupsGatingOnlyTotal.WithLabelValues(state).Inc()
}
