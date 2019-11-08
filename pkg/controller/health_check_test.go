package controller_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/controller"
	"github.com/codeready-toolchain/registration-service/test"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestHealthCheckSuite struct {
	test.UnitTestSuite
}

func TestRunHealthCheckSuite(t *testing.T) {
	suite.Run(t, &TestHealthCheckSuite{test.UnitTestSuite{}})
}

func (s *TestHealthCheckSuite) TestHealthCheckHandler() {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest(http.MethodGet, "/api/v1/health", nil)
	require.NoError(s.T(), err)

	// Check if the config is set to testing mode, so the handler may use this.
	assert.True(s.T(), s.Config.IsTestingMode(), "testing mode not set correctly to true")

	// Create health check instance.
	healthCheckCtrl := controller.NewHealthCheck(s.Config, controller.NewHealthChecker(s.Config))
	handler := gin.HandlerFunc(healthCheckCtrl.GetHandler)

	s.Run("health in testing mode", func() {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		// Setting unit-tests environment
		s.Config.GetViperInstance().Set("environment", configuration.UnitTestsEnvironment)

		handler(ctx)

		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusOK, rr.Code, "handler returned wrong status code")

		// Check the response body is what we expect.
		data := &controller.Health{}
		err := json.Unmarshal(rr.Body.Bytes(), &data)
		require.NoError(s.T(), err)

		assertHealth(s.T(), true, "unit-tests", data)
	})

	s.Run("health in production mode", func() {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		// Setting production mode
		s.Config.GetViperInstance().Set("environment", "prod")
		assert.False(s.T(), s.Config.IsTestingMode(), "testing mode not set correctly to false")

		// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
		// directly and pass in our Request and ResponseRecorder.
		handler(ctx)

		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusOK, rr.Code, "handler returned wrong status code")

		// Check the response body is what we expect.
		data := &controller.Health{}
		err := json.Unmarshal(rr.Body.Bytes(), &data)
		require.NoError(s.T(), err)

		assertHealth(s.T(), true, "prod", data)
	})

	s.Run("service Unavailable", func() {
		// Setting production mode
		s.Config.GetViperInstance().Set("environment", "testServiceUnavailable")

		healthCheckCtrl := controller.NewHealthCheck(s.Config, &mockHealthChecker{})
		handler := gin.HandlerFunc(healthCheckCtrl.GetHandler)

		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		handler(ctx)

		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusServiceUnavailable, rr.Code, "handler returned wrong status code")

		// Check the response body is what we expect.
		data := &controller.Health{}
		err := json.Unmarshal(rr.Body.Bytes(), &data)
		require.NoError(s.T(), err)

		assertHealth(s.T(), false, "testServiceUnavailable", data)
	})
}

func assertHealth(t *testing.T, expectedAlive bool, expectedEnvironment string, actual *controller.Health) {
	assert.Equal(t, expectedAlive, actual.Alive, "wrong alive in health response")
	assert.Equal(t, configuration.Commit, actual.Revision, "wrong revision in health response")
	assert.Equal(t, configuration.BuildTime, actual.BuildTime, "wrong build_time in health response")
	assert.Equal(t, configuration.StartTime, actual.StartTime, "wrong start_time in health response")
	assert.Equal(t, expectedEnvironment, actual.Environment, "wrong environment in health response")
}

type mockHealthChecker struct {
	alive bool
}

func (c *mockHealthChecker) Alive() bool {
	return c.alive
}
