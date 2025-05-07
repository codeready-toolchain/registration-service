package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	testconfig "github.com/codeready-toolchain/toolchain-common/pkg/test/config"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/test"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestUIConfigSuite struct {
	test.UnitTestSuite
}

func TestRunUIConfigSuite(t *testing.T) {
	suite.Run(t, &TestUIConfigSuite{test.UnitTestSuite{}})
}

func (s *TestUIConfigSuite) TestUIConfigHandler() {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest(http.MethodGet, "/api/v1/uiconfig", nil)
	require.NoError(s.T(), err)

	// Check if the config is set to testing mode, so the handler may use this.
	assert.True(s.T(), configuration.IsTestingMode(), "testing mode not set correctly to true")
	s.OverrideApplicationDefault(testconfig.RegistrationService().
		RegistrationServiceURL("https://signup.domain.com"))
	defer s.DefaultConfig()
	cfg := configuration.GetRegistrationServiceConfig()

	// Create handler instance.
	uiConfigCtrl := NewUIConfig()
	handler := gin.HandlerFunc(uiConfigCtrl.GetHandler)

	s.Run("valid json config", func() {

		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		handler(ctx)

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusOK, rr.Code)

		// Check the response body is what we expect.
		// get config values from endpoint response
		var data *UIConfigResponse
		err = json.Unmarshal(rr.Body.Bytes(), &data)
		require.NoError(s.T(), err)

		s.Run("uiCanaryDeploymentWeight", func() {
			assert.Equal(s.T(), cfg.UICanaryDeploymentWeight(), data.UICanaryDeploymentWeight, "wrong 'UICanaryDeploymentWeight' in uiconfig response")
		})
	})
}
