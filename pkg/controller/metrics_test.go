package controller

import (
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/metrics"
	"github.com/codeready-toolchain/registration-service/test"
	"github.com/gin-gonic/gin"
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
	req, err := http.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
	require.NoError(s.T(), err)

	// Check if the config is set to testing mode, so the handler may use this.
	assert.True(s.T(), configuration.IsTestingMode(), "testing mode not set correctly to true")

	// Create handler instance.
	metricsCtrl := NewMetrics()
	metrics.RegisterCustomMetrics()
	handler := gin.HandlerFunc(metricsCtrl.PrometheusHandler)

	s.Run("valid metrics json", func() {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		handler(ctx)

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusOK, rr.Code)

		// check response content-type.
		require.Equal(s.T(), "text/plain; version=0.0.4; charset=utf-8", rr.Header().Get("Content-Type"))

	})
}
