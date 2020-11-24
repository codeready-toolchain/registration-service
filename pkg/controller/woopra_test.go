package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/codeready-toolchain/registration-service/test"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestWoopraSuite struct {
	test.UnitTestSuite
}

func TestRunWoopraSuite(t *testing.T) {
	suite.Run(t, &TestAuthConfigSuite{test.UnitTestSuite{}})
}

func (s *TestAuthConfigSuite) TestWoopraHandler() {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest(http.MethodGet, "/api/v1/woopra", nil)
	require.NoError(s.T(), err)

	// Check if the config is set to testing mode, so the handler may use this.
	assert.True(s.T(), s.Config().IsTestingMode(), "testing mode not set correctly to true")

	// Create handler instance.

	woopraCtrl := NewWoopra(s.Config())
	handler := gin.HandlerFunc(woopraCtrl.GetWoopraDomain)

	s.Run("valid woopra json", func() {

		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req
		s.ViperConfig().GetViperInstance().Set("WOOPRA_DOMAIN", "testing woopra domain")
		assert.Equal(s.T(), "testing woopra domain", s.Config().GetWoopraDomain())
		handler(ctx)

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusOK, rr.Code)

		// Check the response body is what we expect.
		// get config values from endpoint response
		var dataEnvelope *woopraResponse
		err = json.Unmarshal(rr.Body.Bytes(), &dataEnvelope)
		require.NoError(s.T(), err)

		s.Run("envelope woopra domain name", func() {
			assert.Equal(s.T(), s.Config().GetWoopraDomain(), dataEnvelope.WoopraDomain, "wrong 'woopra domain name' in woopra response")
		})
	})
}
