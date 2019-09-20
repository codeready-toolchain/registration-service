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
	testutils "github.com/codeready-toolchain/registration-service/test"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)
type TestHealthCheckSuite struct {
	testutils.UnitTestSuite
}

 func TestRunHealthCheckSuite(t *testing.T) {
	suite.Run(t, &TestHealthCheckSuite{testutils.UnitTestSuite{}})
}

func (s *TestHealthCheckSuite) TestHealthCheckHandler() {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest(http.MethodGet, "/api/v1/health", nil)
	require.NoError(s.T(), err)

	// Create logger and registry.
	logger := log.New(os.Stderr, "", 0)

	// Check if the config is set to testing mode, so the handler may use this.
	assert.True(s.T(), s.Config.IsTestingMode(), "testing mode not set correctly to true")

	// Create health check instance.
	healthCheckCtrl := controller.NewHealthCheck(logger, s.Config)
	handler := gin.HandlerFunc(healthCheckCtrl.GetHandler)

	s.Run("health in testing mode", func() {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		handler(ctx)

		// Check the status code is what we expect.
		assert.Equal(s.T(), rr.Code, http.StatusOK, "handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)

		// Check the response body is what we expect.
		var data map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &data)
		require.NoError(s.T(), err)

		val, ok := data["alive"]
		assert.True(s.T(), ok, "no alive key in health response")
		valBool, ok := val.(bool)
		assert.True(s.T(), ok, "returned 'alive' value is not of type 'bool'")
		assert.True(s.T(), valBool, "alive is false in test mode health response")
	})

	s.Run("health in production mode", func() {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		// Setting production mode
		s.Config.GetViperInstance().Set("testingmode", false)
		assert.False(s.T(), s.Config.IsTestingMode(), "testing mode not set correctly to false")

		// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
		// directly and pass in our Request and ResponseRecorder.
		handler(ctx)

		// Check the status code is what we expect.
		assert.Equal(s.T(), rr.Code, http.StatusOK, "handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)

		// Check the response body is what we expect.
		var data map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &data)
		require.NoError(s.T(), err)

		val, ok := data["alive"]
		assert.True(s.T(), ok, "no alive key in health response")
		valBool, ok := val.(bool)
		assert.True(s.T(), ok, "returned 'alive' value is not of type 'bool'")
		assert.True(s.T(), valBool, "alive is false in test mode health response")
	})

	s.Run("revision", func() {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
		// directly and pass in our Request and ResponseRecorder.
		handler(ctx)

		// Check the status code is what we expect.
		assert.Equal(s.T(), rr.Code, http.StatusOK, "handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)

		// Check the response body is what we expect.
		var data map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &data)
		require.NoError(s.T(), err)

		val, ok := data["revision"]
		assert.True(s.T(), ok, "no revision key in health response")
		valString, ok := val.(string)
		assert.True(s.T(), ok, "returned 'revision' value is not of type 'string'")
		assert.Equal(s.T(), configuration.Commit, valString, "wrong revision in health response, got %s want %s", valString, configuration.Commit)
	})

	s.Run("build time", func() {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
		// directly and pass in our Request and ResponseRecorder.
		handler(ctx)

		// Check the status code is what we expect.
		assert.Equal(s.T(), rr.Code, http.StatusOK, "handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)

		// Check the response body is what we expect.
		var data map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &data)
		require.NoError(s.T(), err)

		val, ok := data["build_time"]
		assert.True(s.T(), ok, "no build_time key in health response")
		valString, ok := val.(string)
		assert.True(s.T(), ok, "returned 'build_time' value is not of type 'string'")
		assert.Equal(s.T(), configuration.BuildTime, valString, "wrong build_time in health response, got %s want %s", valString, configuration.BuildTime)
	})

	s.Run("start time", func() {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
		// directly and pass in our Request and ResponseRecorder.
		handler(ctx)

		// Check the status code is what we expect.
		assert.Equal(s.T(), rr.Code, http.StatusOK, "handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)

		// Check the response body is what we expect.
		var data map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &data)
		require.NoError(s.T(), err)

		val, ok := data["start_time"]
		assert.True(s.T(), ok, "no start_time key in health response")
		valString, ok := val.(string)
		assert.True(s.T(), ok, "returned 'start_time' value is not of type 'string'")
		assert.Equal(s.T(), configuration.StartTime, valString, "wrong start_time in health response, got %s want %s", valString, configuration.StartTime)
	})
}
