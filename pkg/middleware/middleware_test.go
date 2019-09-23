package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/server"
	testutils "github.com/codeready-toolchain/registration-service/test"
	uuid "github.com/satori/go.uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthMiddleware(t *testing.T) {

	// create a TokenGenerator and a key
	tokengenerator := testutils.NewTokenManager()
	kid0 := uuid.NewV4().String()
	_, err := tokengenerator.AddPrivateKey(kid0)
	require.NoError(t, err)

	// create some test tokens
	identity0 := testutils.Identity {
		ID: uuid.NewV4(),
		Username: uuid.NewV4().String(),
	}
	emailClaim0 := testutils.WithEmailClaim(uuid.NewV4().String() + "@email.tld")
	tokenValid, err := tokengenerator.GenerateSignedToken(identity0, kid0, emailClaim0)
	require.NoError(t, err)
	tokenInvalidNoEmail, err := tokengenerator.GenerateSignedToken(identity0, kid0)
	require.NoError(t, err)
	tokenInvalidGarbage := uuid.NewV4().String()

	// start key service
	keysEndpointURL := tokengenerator.NewKeyServer().URL

	// create server
	srv, err := server.New("")
	require.NoError(t, err)	

	// set the key service url in the config
	os.Setenv(configuration.EnvPrefix+"_"+"AUTH_CLIENT_PUBLIC_KEYS_URL", keysEndpointURL)
	assert.Equal(t, keysEndpointURL, srv.Config().GetAuthClientPublicKeysURL(), "key url not set correctly")
	os.Setenv(configuration.EnvPrefix+"_"+"TESTINGMODE", "true")
	assert.True(t, srv.Config().IsTestingMode(), "testing mode not set correctly")
	
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
		name    		string
		urlPath 		string
		method  		string
		tokenHeader	string
		status  		int
	}{
		{"static, no auth", "/favicon.ico", "GET", "", http.StatusOK},
		{"health, no auth", "/api/v1/health", "GET", "", http.StatusOK},
		{"health_private, no auth, denied", "/api/v1/health_private", "GET", "", http.StatusForbidden},
		{"health_private, valid header auth", "/api/v1/health_private", "GET", "Bearer " + tokenValid, http.StatusOK},
		{"health_private, invalid header auth, no email claim", "/api/v1/health_private", "GET", "Bearer " + tokenInvalidNoEmail, http.StatusForbidden},
		{"health_private, invalid header auth, token garbage", "/api/v1/health_private", "GET", "Bearer " + tokenInvalidGarbage, http.StatusForbidden},
		{"health_private, invalid header auth, wrong header format", "/api/v1/health_private", "GET", tokenValid, http.StatusForbidden},
		{"health_private, valid param auth", "/api/v1/health_private?token=" + tokenValid, "GET", "", http.StatusOK},
		{"health_private, invalid param auth, no email claim", "/api/v1/health_private?token=" + tokenInvalidNoEmail, "GET", "", http.StatusForbidden},
		{"health_private, invalid param auth, token garbage", "/api/v1/health_private?token=" + tokenInvalidGarbage, "GET", "", http.StatusForbidden},
	}
	for _, tt := range authtests {
		t.Run(tt.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			req, err := http.NewRequest(tt.method, tt.urlPath, nil)
			require.NoError(t, err)
			if tt.tokenHeader != "" {
				req.Header.Set("Authorization", tt.tokenHeader)
			}
			srv.Engine().ServeHTTP(resp, req)
			// Check the status code is what we expect.
			assert.Equal(t, tt.status, resp.Code, "request returned wrong status code: got %v want %v", resp.Code, tt.status)
		})
	}
}
