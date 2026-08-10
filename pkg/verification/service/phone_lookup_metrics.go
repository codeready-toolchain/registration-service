package service

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	PhoneLookupResultAllowed = "allowed"
	PhoneLookupResultBlocked = "blocked"
)

var (
	// PhoneLookupTotal counts successful Twilio Lookup API responses by result and risk category.
	// result=blocked is incremented only when the signup is actually rejected (mode=enabled).
	// result=allowed covers low-risk numbers and high-risk detections in log mode.
	PhoneLookupTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sandbox_signup_phone_lookup_total",
			Help: "Total successful phone lookup operations by result and risk category. result=blocked means the signup was rejected.",
		},
		[]string{"result", "risk_category"},
	)

	// PhoneLookupErrorsTotal counts Twilio Lookup API failures (fail-open cases).
	// error_type is the HTTP status code when available (e.g. "500", "503"), otherwise "unknown".
	PhoneLookupErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sandbox_signup_phone_lookup_errors_total",
			Help: "Phone lookup API errors (fail-open cases), labeled by HTTP status code when available.",
		},
		[]string{"error_type"},
	)
)

// RegisterPhoneLookupMetrics registers phone lookup metrics with the given registry.
func RegisterPhoneLookupMetrics(reg *prometheus.Registry) {
	reg.MustRegister(PhoneLookupTotal, PhoneLookupErrorsTotal)
}
