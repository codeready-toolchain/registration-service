package authconfig_test

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
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

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Request = req

	handler(ctx)

	// Check the status code is what we expect.
	require.Equal(t, http.StatusOK, rr.Code)

	// Check the response body is what we expect.
	var data map[string]interface{}
	err = json.Unmarshal(rr.Body.Bytes(), &data)
	require.NoError(t, err)

	t.Run("realm", func(t *testing.T) {
		val, ok := data["realm"]
		assert.True(t, ok, "no 'realm' key in authconfig response")
		valString, ok := val.(string)
		assert.True(t, ok, "returned 'realm' value is not of type 'string'")
		assert.Equal(t, configRegistry.GetAuthClientConfigRealm(), valString, "wrong 'realm' in authconfig response, got %s want %s", valString, configRegistry.GetAuthClientConfigRealm())
	})

	t.Run("auth-server-url", func(t *testing.T) {
		val, ok := data["auth-server-url"]
		assert.True(t, ok, "no 'auth-server-url' key in authconfig response")
		valString, ok := val.(string)
		assert.True(t, ok, "returned 'auth-server-url' value is not of type 'string'")
		assert.Equal(t, configRegistry.GetAuthClientConfigAuthServerURL(), valString, "wrong 'auth-server-url' in authconfig response, got %s want %s", valString, configRegistry.GetAuthClientConfigAuthServerURL())
	})

	t.Run("ssl-required", func(t *testing.T) {
		val, ok := data["ssl-required"]
		assert.True(t, ok, "no 'ssl-required' key in authconfig response")
		valString, ok := val.(string)
		assert.True(t, ok, "returned 'ssl-required' value is not of type 'string'")
		assert.Equal(t, configRegistry.GetAuthClientConfigSSLRequired(), valString, "wrong 'ssl-required' in authconfig response, got %s want %s", valString, configRegistry.GetAuthClientConfigSSLRequired())
	})

	t.Run("resource", func(t *testing.T) {
		val, ok := data["resource"]
		assert.True(t, ok, "no 'resource' key in authconfig response")
		valString, ok := val.(string)
		assert.True(t, ok, "returned 'resource' value is not of type 'string'")
		assert.Equal(t, configRegistry.GetAuthClientConfigResource(), valString, "wrong 'resource' in authconfig response, got %s want %s", valString, configRegistry.GetAuthClientConfigResource())
	})

	t.Run("public-client", func(t *testing.T) {
		val, ok := data["public-client"]
		assert.True(t, ok, "no 'public-client' key in authconfig response")
		valBool, ok := val.(bool)
		assert.True(t, ok, "returned 'public-client' value is not of type 'bool'")
		assert.Equal(t, configRegistry.IsAuthClientConfigPublicClient(), valBool, "wrong 'public-client' in authconfig response, got %s want %s", valBool, configRegistry.IsAuthClientConfigPublicClient())
	})

	t.Run("confidential-port", func(t *testing.T) {
		val, ok := data["confidential-port"]
		log.Println(reflect.TypeOf(val).String())
		assert.True(t, ok, "no 'confidential-port' key in authconfig response")
		valFloat, ok := val.(float64)
		assert.True(t, ok, "returned 'confidential-port' value is not of type 'float64'")
		valInt := int64(valFloat)
		assert.Equal(t, configRegistry.GetAuthClientConfigConfidentialPort(), valInt, "wrong 'confidential-port' in authconfig response, got %s want %s", valInt, configRegistry.GetAuthClientConfigConfidentialPort())
	})
}
