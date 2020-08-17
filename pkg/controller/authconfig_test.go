package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/codeready-toolchain/registration-service/test"

	"github.com/stretchr/testify/suite"
)

type TestAuthConfigSuite struct {
	test.UnitTestSuite
}

func TestRunAuthClientConfigSuite(t *testing.T) {
	suite.Run(t, &TestAuthConfigSuite{test.UnitTestSuite{}})
}

func (s *TestAuthConfigSuite) TestAuthClientConfigHandler() {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest(http.MethodGet, "/api/v1/authconfig", nil)
	require.NoError(s.T(), err)

	// Check if the config is set to testing mode, so the handler may use this.
	assert.True(s.T(), s.Config.IsTestingMode(), "testing mode not set correctly to true")

	// Create handler instance.
	authConfigCtrl := NewAuthConfig(s.Config)
	handler := gin.HandlerFunc(authConfigCtrl.GetHandler)

	s.Run("valid json config", func() {

		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		handler(ctx)

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusOK, rr.Code)

		// check response content-type.
		require.Equal(s.T(), s.Config.GetAuthClientConfigAuthContentType(), rr.Header().Get("Content-Type"))

		// Check the response body is what we expect.
		// get config values from endpoint response
		var dataEnvelope *configResponse
		err = json.Unmarshal(rr.Body.Bytes(), &dataEnvelope)
		require.NoError(s.T(), err)

		s.Run("envelope client url", func() {
			assert.Equal(s.T(), s.Config.GetAuthClientLibraryURL(), dataEnvelope.AuthClientLibraryURL, "wrong 'auth-client-library-url' in authconfig response")
		})

		s.Run("envelope client config", func() {
			assert.NotEmpty(s.T(), dataEnvelope.AuthClientLibraryURL, "no 'auth-client-config' key in authconfig response")
			assert.Equal(s.T(), s.Config.GetAuthClientConfigRawRealm(), dataEnvelope.AuthClientConfigRawRealm, "wrong 'realm' in authconfig response")
			assert.Equal(s.T(), s.Config.GetAuthClientConfigRawAuthServerURL(), dataEnvelope.AuthClientConfigRawServerURL, "wrong 'auth-server-url' in authconfig response")
			assert.Equal(s.T(), s.Config.GetAuthClientConfigRawSSLRequired(), dataEnvelope.AuthClientConfigRawSSLRequired, "wrong 'ssl-required' in authconfig response")
			assert.Equal(s.T(), s.Config.GetAuthClientConfigRawResource(), dataEnvelope.AuthClientConfigRawResource, "wrong 'resource' in authconfig response")
			assert.Equal(s.T(), s.Config.GetAuthClientConfigRawPublicClient(), dataEnvelope.AuthClientConfigRawPublicClient, "wrong 'public-client' in authconfig response")
			assert.Equal(s.T(), s.Config.GetVerificationDailyLimit(), dataEnvelope.VerificationDailyLimit, "wrong 'verification daily limit' in authconfig response")
			assert.Equal(s.T(), s.Config.GetVerificationAttemptsAllowed(), dataEnvelope.VerificationAttemptsAllowed, "wrong 'verification attempts allowed' in authconfig response")
			assert.Equal(s.T(), s.Config.GetVerificationEnabled(), dataEnvelope.VerificationEnabled, "wrong 'verification enabled' in authconfig response")
		})
	})
}
