package authconfig_test

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/authconfig"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthClientConfigHandler(t *testing.T) {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest(http.MethodGet, "/api/v1/authconfig", nil)
	require.NoError(t, err)

	// Create logger and registry.
	logger := log.New(os.Stderr, "", 0)
	configRegistry := configuration.CreateEmptyRegistry()

	// Set the config for testing mode, the handler may use this.
	configRegistry.GetViperInstance().Set("testingmode", true)
	assert.True(t, configRegistry.IsTestingMode(), "testing mode not set correctly to true")

	// Create handler instance.
	authconfigService := authconfig.New(logger, configRegistry)
	handler := gin.HandlerFunc(authconfigService.AuthconfigHandler)

	t.Run("valid json config", func(t *testing.T) {

		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		handler(ctx)

		// Check the status code is what we expect.
		require.Equal(t, http.StatusOK, rr.Code)

		// check response content-type.
		require.Equal(t, configRegistry.GetAuthClientConfigAuthContentType(), rr.Header().Get("Content-Type"))

		// Check the response body is what we expect.
		// get config values from endpoint response
		var data map[string]interface{}
		err = json.Unmarshal(rr.Body.Bytes(), &data)
		require.NoError(t, err)

		// get the configured values
		var config map[string]interface{}
		err = json.Unmarshal([]byte(configRegistry.GetAuthClientConfigAuthRaw()), &config)
		require.NoError(t, err)

		t.Run("realm", func(t *testing.T) {
			val, ok := data["realm"]
			assert.True(t, ok, "no 'realm' key in authconfig response")
			valString, ok := val.(string)
			assert.True(t, ok, "returned 'realm' value is not of type 'string'")
			assert.Equal(t, config["realm"], valString, "wrong 'realm' in authconfig response, got %s want %s", valString, config["realm"])
		})

		t.Run("auth-server-url", func(t *testing.T) {
			val, ok := data["auth-server-url"]
			assert.True(t, ok, "no 'auth-server-url' key in authconfig response")
			valString, ok := val.(string)
			assert.True(t, ok, "returned 'auth-server-url' value is not of type 'string'")
			assert.Equal(t, config["auth-server-url"], valString, "wrong 'auth-server-url' in authconfig response, got %s want %s", valString, config["auth-server-url"])
		})

		t.Run("ssl-required", func(t *testing.T) {
			val, ok := data["ssl-required"]
			assert.True(t, ok, "no 'ssl-required' key in authconfig response")
			valString, ok := val.(string)
			assert.True(t, ok, "returned 'ssl-required' value is not of type 'string'")
			assert.Equal(t, config["ssl-required"], valString, "wrong 'ssl-required' in authconfig response, got %s want %s", valString, config["ssl-required"])
		})

		t.Run("resource", func(t *testing.T) {
			val, ok := data["resource"]
			assert.True(t, ok, "no 'resource' key in authconfig response")
			valString, ok := val.(string)
			assert.True(t, ok, "returned 'resource' value is not of type 'string'")
			assert.Equal(t, config["resource"], valString, "wrong 'resource' in authconfig response, got %s want %s", valString, config["resource"])
		})

		t.Run("public-client", func(t *testing.T) {
			val, ok := data["public-client"]
			assert.True(t, ok, "no 'public-client' key in authconfig response")
			valBool, ok := val.(bool)
			assert.True(t, ok, "returned 'public-client' value is not of type 'bool'")
			assert.Equal(t, config["public-client"], valBool, "wrong 'public-client' in authconfig response, got %s want %s", valBool, config["public-client"])
		})

		t.Run("confidential-port", func(t *testing.T) {
			val, ok := data["confidential-port"]
			assert.True(t, ok, "no 'confidential-port' key in authconfig response")
			valFloat, ok := val.(float64)
			assert.True(t, ok, "returned 'confidential-port' value is not of type 'float64'")
			assert.Equal(t, config["confidential-port"], valFloat, "wrong 'confidential-port' in authconfig response, got %s want %s", valFloat, config["confidential-port"])
		})
	})
}