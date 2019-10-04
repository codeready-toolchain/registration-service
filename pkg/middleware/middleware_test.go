package middleware_test

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/controller"
	"github.com/codeready-toolchain/registration-service/pkg/middleware"
	"github.com/codeready-toolchain/registration-service/pkg/server"
	testutils "github.com/codeready-toolchain/registration-service/test"
	"github.com/dgrijalva/jwt-go"
	uuid "github.com/satori/go.uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestAuthMiddlewareSuite struct {
	testutils.UnitTestSuite
}

func TestRunAuthMiddlewareSuite(t *testing.T) {
	suite.Run(t, &TestAuthMiddlewareSuite{testutils.UnitTestSuite{}})
}

func (s *TestAuthMiddlewareSuite) TestAuthMiddleware() {
	s.Run("create without logger", func() {
		authMiddleware, err := middleware.NewAuthMiddleware(nil)
		require.Nil(s.T(), authMiddleware)
		require.Error(s.T(), err)
		require.Equal(s.T(), "missing parameters for NewAuthMiddleware", err.Error())
	})
	s.Run("create with DefaultTokenParser failing", func() {
		logger := log.New(os.Stderr, "", 0)
		authMiddleware, err := middleware.NewAuthMiddleware(logger)
		require.Nil(s.T(), authMiddleware)
		require.Error(s.T(), err)
		require.Equal(s.T(), "no default TokenParser created, call `InitializeDefaultTokenParser()` first", err.Error())
	})
}

func (s *TestAuthMiddlewareSuite) TestAuthMiddlewareService() {
	// create a TokenGenerator and a key
	tokengenerator := testutils.NewTokenManager()
	kid0 := uuid.NewV4().String()
	_, err := tokengenerator.AddPrivateKey(kid0)
	require.NoError(s.T(), err)

	// create some test tokens
	identity0 := testutils.Identity{
		ID:       uuid.NewV4(),
		Username: uuid.NewV4().String(),
	}
	emailClaim0 := testutils.WithEmailClaim(uuid.NewV4().String() + "@email.tld")
	// valid token
	tokenValid, err := tokengenerator.GenerateSignedToken(identity0, kid0, emailClaim0)
	require.NoError(s.T(), err)
	// invalid token - no email
	tokenInvalidNoEmail, err := tokengenerator.GenerateSignedToken(identity0, kid0)
	require.NoError(s.T(), err)
	// invalid token - garbage
	tokenInvalidGarbage := uuid.NewV4().String()
	// invalid token - expired
	tokenInvalidExpiredJWT := tokengenerator.GenerateToken(identity0, kid0, emailClaim0)
	tDiff := -60 * time.Second
	tokenInvalidExpiredJWT.Claims.(jwt.MapClaims)["exp"] = time.Now().UTC().Add(tDiff).Unix()
	tokenInvalidExpired, err := tokengenerator.SignToken(tokenInvalidExpiredJWT, kid0)
	require.NoError(s.T(), err)

	// start key service
	keysEndpointURL := tokengenerator.NewKeyServer().URL

	// create server
	srv, err := server.New("")
	require.NoError(s.T(), err)

	// set the key service url in the config
	os.Setenv(configuration.EnvPrefix+"_"+"AUTH_CLIENT_PUBLIC_KEYS_URL", keysEndpointURL)
	assert.Equal(s.T(), keysEndpointURL, srv.Config().GetAuthClientPublicKeysURL(), "key url not set correctly")
	os.Setenv(configuration.EnvPrefix+"_"+"TESTINGMODE", "true")
	assert.True(s.T(), srv.Config().IsTestingMode(), "testing mode not set correctly")

	// Setting up the routes.
	err = srv.SetupRoutes()
	require.NoError(s.T(), err)

	// Check that there are routes registered.
	routes := srv.GetRegisteredRoutes()
	require.NotEmpty(s.T(), routes)

	// Check that Engine() returns the router object.
	require.NotNil(s.T(), srv.Engine())

	s.Run("health check requests", func() {
		health := &controller.Health{}
		resp := httptest.NewRecorder()
		req, err := http.NewRequest(http.MethodGet, "/api/v1/health", nil)
		require.NoError(s.T(), err)

		srv.Engine().ServeHTTP(resp, req)
		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusOK, resp.Code, "request returned wrong status code")

		err = json.Unmarshal(resp.Body.Bytes(), health)
		require.NoError(s.T(), err)

		assert.Equal(s.T(), health.Alive, true)
		assert.Equal(s.T(), health.TestingMode, true)
		assert.Equal(s.T(), health.Revision, "0")
		assert.NotEqual(s.T(), health.BuildTime, "")
		assert.NotEqual(s.T(), health.StartTime, "")
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
			{"auth_test, valid param auth", "/api/v1/auth_test?token=" + tokenValid, "GET", "", http.StatusOK},
			{"auth_test, invalid param auth, no email claim", "/api/v1/auth_test?token=" + tokenInvalidNoEmail, "GET", "", http.StatusUnauthorized},
			{"auth_test, invalid param auth, token garbage", "/api/v1/auth_test?token=" + tokenInvalidGarbage, "GET", "", http.StatusUnauthorized},
		}
		for _, tt := range authtests {
			s.Run(tt.name, func() {
				resp := httptest.NewRecorder()
				req, err := http.NewRequest(tt.method, tt.urlPath, nil)
				require.NoError(s.T(), err)
				if tt.tokenHeader != "" {
					req.Header.Set("Authorization", tt.tokenHeader)
				}
				srv.Engine().ServeHTTP(resp, req)
				// Check the status code is what we expect.
				assert.Equal(s.T(), tt.status, resp.Code, "request returned wrong status code")
			})
		}
	})
}
