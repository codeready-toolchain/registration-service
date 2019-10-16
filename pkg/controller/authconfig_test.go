package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	testutils "github.com/codeready-toolchain/registration-service/test"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestAuthConfigSuite struct {
	testutils.UnitTestSuite
}

func TestRunAuthClientConfigSuite(t *testing.T) {
	suite.Run(t, &TestAuthConfigSuite{testutils.UnitTestSuite{}})
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

		// get the configured values
		var config map[string]interface{}
		err = json.Unmarshal([]byte(s.Config.GetAuthClientConfigAuthRaw()), &config)
		require.NoError(s.T(), err)

		s.Run("envelope client url", func() {
			assert.Equal(s.T(), s.Config.GetAuthClientLibraryURL(), dataEnvelope.AuthClientLibraryURL, "wrong 'auth-client-library-url' in authconfig response")
		})

		s.Run("envelope client config", func() {
			assert.NotEmpty(s.T(), dataEnvelope.AuthClientLibraryURL, "no 'auth-client-config' key in authconfig response")

			// deserialize the values in the string field
			var data map[string]interface{}
			err = json.Unmarshal([]byte(dataEnvelope.AuthClientConfigRaw), &data)
			require.NoError(s.T(), err)

			s.Run("realm", func() {
				val, ok := data["realm"]
				assert.True(s.T(), ok, "no 'realm' key in authconfig response")
				valString, ok := val.(string)
				assert.True(s.T(), ok, "returned 'realm' value is not of type 'string'")
				assert.Equal(s.T(), config["realm"], valString, "wrong 'realm' in authconfig response")
			})

			s.Run("auth-server-url", func() {
				val, ok := data["auth-server-url"]
				assert.True(s.T(), ok, "no 'auth-server-url' key in authconfig response")
				valString, ok := val.(string)
				assert.True(s.T(), ok, "returned 'auth-server-url' value is not of type 'string'")
				assert.Equal(s.T(), config["auth-server-url"], valString, "wrong 'auth-server-url' in authconfig response")
			})

			s.Run("ssl-required", func() {
				val, ok := data["ssl-required"]
				assert.True(s.T(), ok, "no 'ssl-required' key in authconfig response")
				valString, ok := val.(string)
				assert.True(s.T(), ok, "returned 'ssl-required' value is not of type 'string'")
				assert.Equal(s.T(), config["ssl-required"], valString, "wrong 'ssl-required' in authconfig response")
			})

			s.Run("resource", func() {
				val, ok := data["resource"]
				assert.True(s.T(), ok, "no 'resource' key in authconfig response")
				valString, ok := val.(string)
				assert.True(s.T(), ok, "returned 'resource' value is not of type 'string'")
				assert.Equal(s.T(), config["resource"], valString, "wrong 'resource' in authconfig response")
			})

			s.Run("public-client", func() {
				val, ok := data["public-client"]
				assert.True(s.T(), ok, "no 'public-client' key in authconfig response")
				valBool, ok := val.(bool)
				assert.True(s.T(), ok, "returned 'public-client' value is not of type 'bool'")
				assert.Equal(s.T(), config["public-client"], valBool, "wrong 'public-client' in authconfig response")
			})

			s.Run("confidential-port", func() {
				val, ok := data["confidential-port"]
				assert.True(s.T(), ok, "no 'confidential-port' key in authconfig response")
				valFloat, ok := val.(float64)
				assert.True(s.T(), ok, "returned 'confidential-port' value is not of type 'float64'")
				assert.Equal(s.T(), config["confidential-port"], valFloat, "wrong 'confidential-port' in authconfig response")
			})
		})
	})
}
