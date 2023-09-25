package handlers

import (
	"github.com/codeready-toolchain/registration-service/pkg/metrics"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMetricsHandler(t *testing.T) {
	// Create a request to pass to our handler. We don't have any query parameters, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest(http.MethodGet, "/metrics", nil)
	require.NoError(t, err)

	// Create handler instance.
	metricsCtrl := NewMetrics()
	metrics.RegisterCustomMetrics()

	t.Run("valid metrics json", func(t *testing.T) {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		e := echo.New()
		rec := httptest.NewRecorder()
		ctx := e.NewContext(req, rec)

		//when
		metricsCtrl.PrometheusHandler(ctx)

		// then
		// check the status code is what we expect.
		require.Equal(t, http.StatusOK, rec.Code)
		// check response content-type.
		require.Equal(t, "text/plain; version=0.0.4; charset=utf-8", rec.Header().Get("Content-Type"))
	})
}
