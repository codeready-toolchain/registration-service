package configuration

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	// Commit current build commit set by build script.
	Commit = "0"
	// BuildTime set by build script in ISO 8601 (UTC) format:
	// YYYY-MM-DDThh:mm:ssTZD (see https://www.w3.org/TR/NOTE-datetime for
	// details).
	BuildTime = "0"
	// StartTime in ISO 8601 (UTC) format.
	StartTime = time.Now().UTC().Format("2006-01-02T15:04:05Z")
)

var (
	// RegistrationServiceShortCommitGaugeVec reflects the current short git commit of the registration service (via the `commit` label)
	RegistrationServiceShortCommitGaugeVec *prometheus.GaugeVec
	// RegistrationServiceCommitGaugeVec reflects the current full git commit of the registration service (via the `commit` label)
	RegistrationServiceCommitGaugeVec *prometheus.GaugeVec
)

func RegisterVersionMetrics(registry *prometheus.Registry) {
	// RegistrationServiceCommitGaugeVec reflects the current full git commit of the registration service (via the `commit` label)
	RegistrationServiceCommitGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "sandbox_registration_service_commit",
		Help: "The commit of the registration service",
	}, []string{"commit"})
	RegistrationServiceCommitGaugeVec.WithLabelValues(Commit).SetToCurrentTime() // automatically set the value to the current time, so that the highest value is the current commit

	// RegistrationServiceShortCommitGaugeVec reflects the current short git commit of the registration service (via the `commit` label)
	RegistrationServiceShortCommitGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "sandbox_registration_service_short_commit",
		Help: "The short commit of the registration service",
	}, []string{"commit"})
	shortCommit := Commit
	if len(Commit) >= 7 {
		shortCommit = Commit[0:7]
	}
	RegistrationServiceShortCommitGaugeVec.WithLabelValues(shortCommit).SetToCurrentTime() // automatically set the value to the current time, so that the highest value is the current commit
	registry.MustRegister(RegistrationServiceCommitGaugeVec, RegistrationServiceShortCommitGaugeVec)
}
