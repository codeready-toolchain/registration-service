package middleware_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/codeready-toolchain/registration-service/test/fake"
	"gopkg.in/h2non/gock.v1"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/middleware"
	"github.com/codeready-toolchain/registration-service/pkg/proxy"
	"github.com/codeready-toolchain/registration-service/pkg/server"
	"github.com/codeready-toolchain/registration-service/test"
	"github.com/codeready-toolchain/toolchain-common/pkg/status"
	authsupport "github.com/codeready-toolchain/toolchain-common/pkg/test/auth"
	testconfig "github.com/codeready-toolchain/toolchain-common/pkg/test/config"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestAuthMiddlewareSuite struct {
	test.UnitTestSuite
}

func TestRunAuthMiddlewareSuite(t *testing.T) {
	suite.Run(t, &TestAuthMiddlewareSuite{test.UnitTestSuite{}})
}

func (s *TestAuthMiddlewareSuite) TestAuthMiddleware() {
	s.Run("create with DefaultTokenParser failing", func() {
		authMiddleware, err := middleware.NewAuthMiddleware()
		require.Nil(s.T(), authMiddleware)
		require.Error(s.T(), err)
		require.Equal(s.T(), "no default TokenParser created, call `InitializeDefaultTokenParser()` first", err.Error())
	})
}

func (s *TestAuthMiddlewareSuite) TestAuthMiddlewareService() {
	// create a TokenGenerator and a key
	tokengenerator := authsupport.NewTokenManager()
	kid0 := uuid.NewString()
	_, err := tokengenerator.AddPrivateKey(kid0)
	require.NoError(s.T(), err)

	// create some test tokens
	identity0 := authsupport.Identity{
		ID:       uuid.New(),
		Username: uuid.NewString(),
	}
	emailClaim0 := authsupport.WithEmailClaim(uuid.NewString() + "@email.tld")
	// valid token
	tokenValid, err := tokengenerator.GenerateSignedToken(identity0, kid0, emailClaim0)
	require.NoError(s.T(), err)
	// invalid token - no email
	tokenInvalidNoEmail, err := tokengenerator.GenerateSignedToken(identity0, kid0)
	require.NoError(s.T(), err)
	// invalid token - garbage
	tokenInvalidGarbage := uuid.NewString()
	// invalid token - expired
	expTime := time.Now().Add(-60 * time.Second)
	expClaim := authsupport.WithExpClaim(expTime)
	tokenInvalidExpiredJWT := tokengenerator.GenerateToken(identity0, kid0, emailClaim0, expClaim)
	tokenInvalidExpired, err := tokengenerator.SignToken(tokenInvalidExpiredJWT, kid0)
	require.NoError(s.T(), err)

	// start key service
	keysEndpointURL := tokengenerator.NewKeyServer().URL

	// create server
	srv := server.New(fake.NewMockableApplication(nil))

	// set the key service url in the config
	s.SetConfig(testconfig.RegistrationService().
		Environment(configuration.UnitTestsEnvironment).
		Auth().AuthClientPublicKeysURL(keysEndpointURL))

	cfg := configuration.GetRegistrationServiceConfig()
	assert.Equal(s.T(), keysEndpointURL, cfg.Auth().AuthClientPublicKeysURL(), "key url not set correctly")

	// Setting up the routes.
	err = srv.SetupRoutes(proxy.DefaultPort)
	require.NoError(s.T(), err)

	// Check that there are routes registered.
	routes := srv.GetRegisteredRoutes()
	require.NotEmpty(s.T(), routes)

	// Check that Engine() returns the router object.
	require.NotNil(s.T(), srv.Engine())

	s.Run("health check requests", func() {
		// given
		health := &status.Health{}
		resp := httptest.NewRecorder()
		req, err := http.NewRequest(http.MethodGet, "/api/v1/health", nil)
		require.NoError(s.T(), err)

		// mock proxy
		defer gock.Off()
		gock.New(fmt.Sprintf("http://localhost:%s/proxyhealth", proxy.DefaultPort)).
			Reply(http.StatusOK).
			BodyString("")

		// when
		srv.Engine().ServeHTTP(resp, req)

		// then
		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusOK, resp.Code, "request returned wrong status code")

		err = json.Unmarshal(resp.Body.Bytes(), health)
		require.NoError(s.T(), err)

		assert.True(s.T(), health.Alive)
		assert.Equal(s.T(), "unit-tests", health.Environment)
		assert.Equal(s.T(), "0", health.Revision)
		assert.NotEmpty(s.T(), health.BuildTime)
		assert.NotEmpty(s.T(), health.StartTime)
	})

	s.Run("static request", func() {
		resp := httptest.NewRecorder()
		req, err := http.NewRequest(http.MethodGet, "/favicon.ico", nil)
		require.NoError(s.T(), err)

		srv.Engine().ServeHTTP(resp, req)
		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusOK, resp.Code, "request returned wrong status code")
	})

	s.Run("auth requests", func() {

		// do some requests
		var authtests = []struct {
			name        string
			urlPath     string
			method      string
			tokenHeader string
			status      int
		}{
			{"auth_test, no auth, denied", "/api/v1/auth_test", http.MethodGet, "", http.StatusUnauthorized},
			{"auth_test, valid header auth", "/api/v1/auth_test", http.MethodGet, "Bearer " + tokenValid, http.StatusOK},
			{"auth_test, invalid header auth, no email claim", "/api/v1/auth_test", http.MethodGet, "Bearer " + tokenInvalidNoEmail, http.StatusUnauthorized},
			{"auth_test, invalid header auth, expired", "/api/v1/auth_test", http.MethodGet, "Bearer " + tokenInvalidExpired, http.StatusUnauthorized},
			{"auth_test, invalid header auth, token garbage", "/api/v1/auth_test", http.MethodGet, "Bearer " + tokenInvalidGarbage, http.StatusUnauthorized},
			{"auth_test, invalid header auth, wrong header format", "/api/v1/auth_test", http.MethodGet, tokenValid, http.StatusUnauthorized},
			{"auth_test, invalid header auth, bearer but no token", "/api/v1/auth_test", http.MethodGet, "Bearer ", http.StatusUnauthorized},
		}
		for _, tt := range authtests {
			s.Run(tt.name, func() {
				// given
				resp := httptest.NewRecorder()
				req, err := http.NewRequest(tt.method, tt.urlPath, nil)
				require.NoError(s.T(), err)
				if tt.tokenHeader != "" {
					req.Header.Set("Authorization", tt.tokenHeader)
				}

				// mock proxy
				defer gock.Off()
				gock.New(fmt.Sprintf("http://localhost:%s/proxyhealth", proxy.DefaultPort)).
					Reply(http.StatusOK).
					BodyString("")

				// when
				srv.Engine().ServeHTTP(resp, req)

				// then
				// Check the status code is what we expect.
				assert.Equal(s.T(), tt.status, resp.Code, "request returned wrong status code")
			})
		}
	})
}
