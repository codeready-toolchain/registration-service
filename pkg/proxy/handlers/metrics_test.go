package handlers

import (
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/metrics"
	"github.com/codeready-toolchain/registration-service/test"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"net/http"
	"net/http/httptest"
	"testing"
)

type TestMetricsSuite struct {
	test.UnitTestSuite
}

func TestRunMetricsSuite(t *testing.T) {
	suite.Run(t, &TestMetricsSuite{test.UnitTestSuite{}})
}

func (s *TestMetricsSuite) TestMetricsHandler() {
	// Create a request to pass to our handler. We don't have any query parameters, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest(http.MethodGet, "/metrics", nil)
	require.NoError(s.T(), err)

	// Check if the config is set to testing mode, so the handler may use this.
	assert.True(s.T(), configuration.IsTestingMode(), "testing mode not set correctly to true")

	// Create handler instance.
	metricsCtrl := NewMetrics()
	metrics.RegisterCustomMetrics()

	s.Run("valid metrics json", func() {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		e := echo.New()
		rec := httptest.NewRecorder()
		ctx := e.NewContext(req, rec)

		err := metricsCtrl.PrometheusHandler(ctx)
		require.NoError(s.T(), err)
		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusOK, rec.Code)

		// check response content-type.
		require.Equal(s.T(), "text/plain; version=0.0.4; charset=utf-8", rec.Header().Get("Content-Type"))

	})
}
