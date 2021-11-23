package controller_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/controller"
	"github.com/codeready-toolchain/registration-service/pkg/proxy"
	"github.com/codeready-toolchain/registration-service/test"
	testconfig "github.com/codeready-toolchain/toolchain-common/pkg/test/config"
	"gopkg.in/h2non/gock.v1"

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
	assert.True(s.T(), configuration.IsTestingMode(), "testing mode not set correctly to true")

	// Create health check instance.
	healthCheckCtrl := controller.NewHealthCheck(controller.NewHealthChecker())
	handler := gin.HandlerFunc(healthCheckCtrl.GetHandler)

	s.Run("health in testing mode", func() {
		// given
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		// Setting unit-tests environment
		s.OverrideApplicationDefault(testconfig.RegistrationService().
			Environment(configuration.UnitTestsEnvironment))

		// mock proxy
		defer gock.Off()
		gock.New(fmt.Sprintf("http://localhost:%s", proxy.ProxyPort)).
			Get("/health").
			Persist().
			Reply(http.StatusOK).
			BodyString("")

		// when
		handler(ctx)

		// then
		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusOK, rr.Code, "handler returned wrong status code")

		// Check the response body is what we expect.
		data := &controller.HealthStatus{}
		err := json.Unmarshal(rr.Body.Bytes(), &data)
		require.NoError(s.T(), err)

		assertHealth(s.T(), true, true, "unit-tests", data)
	})

	s.Run("health in production mode", func() {
		// given
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		// Setting production mode
		s.OverrideApplicationDefault(testconfig.RegistrationService().
			Environment("prod"))
		assert.False(s.T(), configuration.IsTestingMode(), "testing mode not set correctly to false")

		// mock proxy
		defer gock.Off()
		gock.New(fmt.Sprintf("http://localhost:%s", proxy.ProxyPort)).
			Get("/health").
			Persist().
			Reply(http.StatusOK).
			BodyString("")

		// when
		// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
		// directly and pass in our Request and ResponseRecorder.
		handler(ctx)

		// then
		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusOK, rr.Code, "handler returned wrong status code")

		// Check the response body is what we expect.
		data := &controller.HealthStatus{}
		err := json.Unmarshal(rr.Body.Bytes(), &data)
		require.NoError(s.T(), err)

		assertHealth(s.T(), true, true, "prod", data)
	})

	s.Run("service Unavailable due to reg service", func() {
		// Setting production mode
		s.OverrideApplicationDefault(testconfig.RegistrationService().
			Environment("testServiceUnavailable"))

		healthCheckCtrl := controller.NewHealthCheck(&mockHealthChecker{
			alive:      false,
			proxyAlive: true,
		})
		handler := gin.HandlerFunc(healthCheckCtrl.GetHandler)

		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		// mock proxy
		defer gock.Off()
		gock.New(fmt.Sprintf("http://localhost:%s", proxy.ProxyPort)).
			Get("/health").
			Persist().
			Reply(http.StatusOK).
			BodyString("")

		handler(ctx)

		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusServiceUnavailable, rr.Code, "handler returned wrong status code")

		// Check the response body is what we expect.
		data := &controller.HealthStatus{}
		err := json.Unmarshal(rr.Body.Bytes(), &data)
		require.NoError(s.T(), err)

		assertHealth(s.T(), false, true, "testServiceUnavailable", data)
	})

	s.Run("only proxy not available", func() {
		// Setting production mode
		s.OverrideApplicationDefault(testconfig.RegistrationService().
			Environment("testServiceUnavailable"))

		healthCheckCtrl := controller.NewHealthCheck(&mockHealthChecker{
			alive:      true,
			proxyAlive: false,
		})
		handler := gin.HandlerFunc(healthCheckCtrl.GetHandler)

		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		handler(ctx)

		// Check the status code is what we expect. OK is returned as long as status.Alive is true
		assert.Equal(s.T(), http.StatusOK, rr.Code, "handler returned wrong status code")

		// Check the response body is what we expect.
		data := &controller.HealthStatus{}
		err := json.Unmarshal(rr.Body.Bytes(), &data)
		require.NoError(s.T(), err)

		assertHealth(s.T(), true, false, "testServiceUnavailable", data)
	})

	s.Run("service Unavailable due to both reg service and proxy down", func() {
		// Setting production mode
		s.OverrideApplicationDefault(testconfig.RegistrationService().
			Environment("testServiceUnavailable"))

		healthCheckCtrl := controller.NewHealthCheck(&mockHealthChecker{})
		handler := gin.HandlerFunc(healthCheckCtrl.GetHandler)

		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		handler(ctx)

		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusServiceUnavailable, rr.Code, "handler returned wrong status code")

		// Check the response body is what we expect.
		data := &controller.HealthStatus{}
		err := json.Unmarshal(rr.Body.Bytes(), &data)
		require.NoError(s.T(), err)

		assertHealth(s.T(), false, false, "testServiceUnavailable", data)
	})
}

func assertHealth(t *testing.T, expectedAlive, expectedAPIProxyAlive bool, expectedEnvironment string, actual *controller.HealthStatus) {
	assert.Equal(t, expectedAlive, actual.Alive, "wrong alive in health response")
	assert.Equal(t, expectedAPIProxyAlive, actual.ProxyAlive, "wrong API proxy alive in health response")
	assert.Equal(t, configuration.Commit, actual.Revision, "wrong revision in health response")
	assert.Equal(t, configuration.BuildTime, actual.BuildTime, "wrong build_time in health response")
	assert.Equal(t, configuration.StartTime, actual.StartTime, "wrong start_time in health response")
	assert.Equal(t, expectedEnvironment, actual.Environment, "wrong environment in health response")
}

type mockHealthChecker struct {
	alive      bool
	proxyAlive bool
}

func (c *mockHealthChecker) Alive(ctx *gin.Context) bool {
	return c.alive
}

func (c *mockHealthChecker) APIProxyAlive(ctx *gin.Context) bool {
	return c.proxyAlive
}
