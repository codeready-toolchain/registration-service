package service

import (
	"strconv"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/toolchain-common/pkg/states"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	PhoneLookupResultAllowed = "allowed"
	PhoneLookupResultBlocked = "blocked"
)

var (
	// PhoneLookupTotal counts successful Twilio Lookup API responses by result, risk category, and no-provisioning state.
	// result=blocked is incremented only when the signup is actually rejected (mode=enabled).
	// result=allowed covers low-risk numbers and high-risk detections in log mode.
	PhoneLookupTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sandbox_signup_phone_lookup_total",
			Help: "Total successful phone lookup operations by result, risk category, and no-provisioning state. result=blocked means the signup was rejected.",
		},
		[]string{"result", "risk_category", "no_provisioning"},
	)

	// PhoneLookupErrorsTotal counts Twilio Lookup API failures (fail-open cases).
	// error_type is the HTTP status code when available (e.g. "500", "503"), otherwise "unknown".
	PhoneLookupErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sandbox_signup_phone_lookup_errors_total",
			Help: "Phone lookup API errors (fail-open cases), labeled by HTTP status code when available and no-provisioning state.",
		},
		[]string{"error_type", "no_provisioning"},
	)
)

// noProvisioningLabel returns "true" or "false" from UserSignup spec state (never omit the Prometheus label).
func noProvisioningLabel(signup *toolchainv1alpha1.UserSignup) string {
	return strconv.FormatBool(states.NoProvisioning(signup))
}

// RegisterPhoneLookupMetrics registers phone lookup metrics with the given registry.
func RegisterPhoneLookupMetrics(reg *prometheus.Registry) {
	reg.MustRegister(PhoneLookupTotal, PhoneLookupErrorsTotal)
}
