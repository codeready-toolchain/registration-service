package controller_test

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/controller"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthCheckHandler(t *testing.T) {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest(http.MethodGet, "/api/v1/health", nil)
	require.NoError(t, err)

	// Create logger and registry.
	logger := log.New(os.Stderr, "", 0)
	configRegistry := configuration.CreateEmptyRegistry()

	// Set the config for testing mode, the handler may use this.
	configRegistry.GetViperInstance().Set("testingmode", true)
	assert.True(t, configRegistry.IsTestingMode(), "testing mode not set correctly to true")

	// Create health check instance.
	healthCheckCtrl := controller.NewHealthCheck(logger, configRegistry)
	handler := gin.HandlerFunc(healthCheckCtrl.GetHandler)

	t.Run("health in testing mode", func(t *testing.T) {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		handler(ctx)

		// Check the status code is what we expect.
		assert.Equal(t, rr.Code, http.StatusInternalServerError, "handler returned wrong status code: got %v want %v", rr.Code, http.StatusInternalServerError)

		// Check the response body is what we expect.
		var data map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &data)
		require.NoError(t, err)

		val, ok := data["alive"]
		assert.True(t, ok, "no alive key in health response")
		valBool, ok := val.(bool)
		assert.True(t, ok, "returned 'alive' value is not of type 'bool'")
		assert.False(t, valBool, "alive is true in test mode health response")
	})

	t.Run("health in production mode", func(t *testing.T) {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		// Setting production mode
		configRegistry.GetViperInstance().Set("testingmode", false)
		assert.False(t, configRegistry.IsTestingMode(), "testing mode not set correctly to false")

		// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
		// directly and pass in our Request and ResponseRecorder.
		handler(ctx)

		// Check the status code is what we expect.
		assert.Equal(t, rr.Code, http.StatusOK, "handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)

		// Check the response body is what we expect.
		var data map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &data)
		require.NoError(t, err)

		val, ok := data["alive"]
		assert.True(t, ok, "no alive key in health response")
		valBool, ok := val.(bool)
		assert.True(t, ok, "returned 'alive' value is not of type 'bool'")
		assert.True(t, valBool, "alive is false in test mode health response")
	})

	t.Run("revision", func(t *testing.T) {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
		// directly and pass in our Request and ResponseRecorder.
		handler(ctx)

		// Check the status code is what we expect.
		assert.Equal(t, rr.Code, http.StatusOK, "handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)

		// Check the response body is what we expect.
		var data map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &data)
		require.NoError(t, err)

		val, ok := data["revision"]
		assert.True(t, ok, "no revision key in health response")
		valString, ok := val.(string)
		assert.True(t, ok, "returned 'revision' value is not of type 'string'")
		assert.Equal(t, configuration.Commit, valString, "wrong revision in health response, got %s want %s", valString, configuration.Commit)
	})

	t.Run("build time", func(t *testing.T) {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
		// directly and pass in our Request and ResponseRecorder.
		handler(ctx)

		// Check the status code is what we expect.
		assert.Equal(t, rr.Code, http.StatusOK, "handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)

		// Check the response body is what we expect.
		var data map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &data)
		require.NoError(t, err)

		val, ok := data["build_time"]
		assert.True(t, ok, "no build_time key in health response")
		valString, ok := val.(string)
		assert.True(t, ok, "returned 'build_time' value is not of type 'string'")
		assert.Equal(t, configuration.BuildTime, valString, "wrong build_time in health response, got %s want %s", valString, configuration.BuildTime)
	})

	t.Run("start time", func(t *testing.T) {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
		// directly and pass in our Request and ResponseRecorder.
		handler(ctx)

		// Check the status code is what we expect.
		assert.Equal(t, rr.Code, http.StatusOK, "handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)

		// Check the response body is what we expect.
		var data map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &data)
		require.NoError(t, err)

		val, ok := data["start_time"]
		assert.True(t, ok, "no start_time key in health response")
		valString, ok := val.(string)
		assert.True(t, ok, "returned 'start_time' value is not of type 'string'")
		assert.Equal(t, configuration.StartTime, valString, "wrong start_time in health response, got %s want %s", valString, configuration.StartTime)
	})
}
