package controller_test

import (
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	testutils "github.com/codeready-toolchain/registration-service/test"
	"github.com/codeready-toolchain/registration-service/pkg/controller"
	
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignupHandler(t *testing.T) {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest(http.MethodPost, "/api/v1/signup", nil)
	require.NoError(t, err)

	// Create logger and registry.
	logger := log.New(os.Stderr, "", 0)
	configRegistry := configuration.CreateEmptyRegistry()

	// Set the config for testing mode, the handler may use this.
	configRegistry.GetViperInstance().Set("testingmode", true)
	assert.True(t, configRegistry.IsTestingMode(), "testing mode not set correctly to true")

	// startup public key service
	tokengenerator := testutils.NewTokenGenerator()
	keysEndpointURL := tokengenerator.GetKeyService()
	// set the key service url in the config
	configRegistry.GetViperInstance().Set("auth_client.public_keys_url", keysEndpointURL)
	assert.Equal(t, keysEndpointURL, configRegistry.GetAuthClientPublicKeysURL(), "key url not set correctly")

	t.Run("signup handler", func(t *testing.T) {
		// Create signup instance.
		signupCtrl := controller.NewSignup(logger, configRegistry)
		handler := gin.HandlerFunc(signupCtrl.PostHandler)

		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		handler(ctx)

		// Check the status code is what we expect.
		require.Equal(t, http.StatusOK, rr.Code)
	})
}
