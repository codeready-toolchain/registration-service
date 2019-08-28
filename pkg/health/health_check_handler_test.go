package health_test

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/health"
	"github.com/stretchr/testify/assert"
)

func TestHealthCheckHandler(t *testing.T) {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest("GET", "/api/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	// create logger and registry.
	logger := log.New(os.Stderr, "", 0)
	configRegistry := configuration.CreateEmptyRegistry()

	// set the config for testing mode, the handler may use this.
	configRegistry.GetViperInstance().Set("testingmode", true)
	assert.True(t, configRegistry.IsTestingMode(), "testing mode not set correctly to true")

	// create handler instance.
	healthService := health.New(logger, configRegistry)
	handler := http.HandlerFunc(healthService.HealthCheckHandler)

	t.Run("health in testing mode", func(t *testing.T) {	
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
		// directly and pass in our Request and ResponseRecorder.
		handler.ServeHTTP(rr, req)
		// Check the status code is what we expect.
		assert.Equal(t, rr.Code, http.StatusInternalServerError, "handler returned wrong status code: got %v want %v", rr.Code, http.StatusInternalServerError)
		// Check the response body is what we expect.
		var data map[string]interface{}
		if err := json.Unmarshal(rr.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
		val, ok := data["alive"]
		assert.True(t, ok, "no alive key in health response")
		valBool, ok := val.(bool)
		assert.True(t, ok, "returned 'alive' value is not of type 'bool'")
		assert.False(t, valBool, "alive is true in test mode health response")
	})

	t.Run("health in production mode", func(t *testing.T) {	
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		// setting production mode
		configRegistry.GetViperInstance().Set("testingmode", false)
		assert.False(t, configRegistry.IsTestingMode(), "testing mode not set correctly to false")
		// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
		// directly and pass in our Request and ResponseRecorder.
		handler.ServeHTTP(rr, req)
		// Check the status code is what we expect.
		assert.Equal(t, rr.Code, http.StatusOK, "handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
		// Check the response body is what we expect.
		var data map[string]interface{}
		if err := json.Unmarshal(rr.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
		val, ok := data["alive"]
		assert.True(t, ok, "no alive key in health response")
		valBool, ok := val.(bool)
		assert.True(t, ok, "returned 'alive' value is not of type 'bool'")
		assert.True(t, valBool, "alive is false in test mode health response")
	})


}
