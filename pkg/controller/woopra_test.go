package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/test"

	testconfig "github.com/codeready-toolchain/toolchain-common/pkg/test/config"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestWoopraSuite struct {
	test.UnitTestSuite
}

func TestRunWoopraSuite(t *testing.T) {
	suite.Run(t, &TestWoopraSuite{test.UnitTestSuite{}})
}

func (s *TestWoopraSuite) TestWoopraHandler() {
	// Check if the config is set to testing mode, so the handler may use this.
	assert.True(s.T(), configuration.IsTestingMode(), "testing mode not set correctly to true")

	// Create handler instance.

	woopraCtrl := NewWoopra()

	s.Run("valid woopra json", func() {

		// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
		// pass 'nil' as the third parameter.
		req, err := http.NewRequest(http.MethodGet, "/api/v1/woopra-domain", nil)
		require.NoError(s.T(), err)

		handler := gin.HandlerFunc(woopraCtrl.GetWoopraDomain)

		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req
		s.OverrideApplicationDefault(testconfig.RegistrationService().
			Analytics().WoopraDomain("testing woopra domain"))

		cfg := configuration.GetRegistrationServiceConfig()

		assert.Equal(s.T(), "testing woopra domain", cfg.Analytics().WoopraDomain())
		handler(ctx)

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusOK, rr.Code)

		// Check the response body is what we expect.
		// get config values from endpoint response
		dataEnvelope := rr.Body.String()
		require.NoError(s.T(), err)

		s.Run("envelope woopra domain name", func() {
			assert.Equal(s.T(), cfg.Analytics().WoopraDomain(), dataEnvelope, "wrong 'woopra domain name' in woopra response")
		})
	})

	s.Run("valid segment write key json", func() {

		// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
		// pass 'nil' as the third parameter.
		req, err := http.NewRequest(http.MethodGet, "/api/v1/segment-write-key", nil)
		require.NoError(s.T(), err)

		handler := gin.HandlerFunc(woopraCtrl.GetSegmentWriteKey)

		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		s.OverrideApplicationDefault(testconfig.RegistrationService().
			Analytics().SegmentWriteKey("testing segment write key"))

		cfg := configuration.GetRegistrationServiceConfig()

		assert.Equal(s.T(), "testing segment write key", cfg.Analytics().SegmentWriteKey())

		handler(ctx)

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusOK, rr.Code)

		// Check the response body is what we expect.
		// get config values from endpoint response
		dataEnvelope := rr.Body.String()
		require.NoError(s.T(), err)

		s.Run("envelope segment write key", func() {
			assert.Equal(s.T(), cfg.Analytics().SegmentWriteKey(), dataEnvelope, "wrong 'segment write key' in segment response")
		})
	})
}
