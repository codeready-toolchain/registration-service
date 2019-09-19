package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/server"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	// We're using the example config for the configuration here as the
	// specific config params do not matter for testing the routes setup.
	srv, err := server.New("../../example-config.yml")
	require.NoError(t, err)

	// Setting up the routes.
	err = srv.SetupRoutes()
	require.NoError(t, err)

	// Check that there are routes registered.
	routes := srv.GetRegisteredRoutes()
	require.NotEmpty(t, routes)

	// Check that Engine() returns the router object.
	require.NotNil(t, srv.Engine())
}

func TestAuthMiddleware(t *testing.T) {
	// We're using the example config for the configuration here as the
	// specific config params do not matter for testing the routes setup.
	srv, err := server.New("../../example-config.yml")
	require.NoError(t, err)

	// Setting up the routes.
	err = srv.SetupRoutes()
	require.NoError(t, err)

	// Check that there are routes registered.
	routes := srv.GetRegisteredRoutes()
	require.NotEmpty(t, routes)

	// Check that Engine() returns the router object.
	require.NotNil(t, srv.Engine())

	// do some requests
	var authtests = []struct {
		name    string
		urlPath string
		method  string
		status  int
	}{
		{"static, no auth", "/favicon.ico", "GET", http.StatusOK},
		{"health, no auth", "/api/v1/health", "GET", http.StatusOK},
		{"signup, auth", "/api/v1/signup", "GET", http.StatusOK},
	}
	for _, tt := range authtests {
		t.Run(tt.name, func(t *testing.T) {

			resp := httptest.NewRecorder()
			req, err := http.NewRequest(tt.method, tt.urlPath, nil)
			require.NoError(t, err)
			srv.Engine().ServeHTTP(resp, req)

			// Check the status code is what we expect.
			assert.Equal(t, tt.status, resp.Code, "request returned wrong status code: got %v want %v", resp.Code, tt.status)
		})
	}
}
