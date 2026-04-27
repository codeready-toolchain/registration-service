package configuration_test

import (
	"testing"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"

	"github.com/prometheus/client_golang/prometheus"
	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestVersionMetrics(t *testing.T) {

	testData := []struct {
		name        string
		commit      string
		shortCommit string
	}{
		{name: "when commit is longer than 7 characters", commit: "short12-34567890", shortCommit: "short12"},
		{name: "when commit is shorter than 7 characters", commit: "short", shortCommit: "short"},
		{name: "when commit is empty", commit: "", shortCommit: ""},
	}

	for _, test := range testData {
		t.Run(test.name, func(t *testing.T) {
			// given
			configuration.Commit = test.commit
			registry := prometheus.NewRegistry()

			// when
			configuration.RegisterVersionMetrics(registry)

			// then
			assert.InDelta(t, float64(time.Now().Unix()), promtestutil.ToFloat64(configuration.RegistrationServiceShortCommitGaugeVec.WithLabelValues(test.shortCommit)), float64(time.Minute.Seconds()))
			assert.InDelta(t, float64(time.Now().Unix()), promtestutil.ToFloat64(configuration.RegistrationServiceCommitGaugeVec.WithLabelValues(test.commit)), float64(time.Minute.Seconds()))
		})
	}

}
