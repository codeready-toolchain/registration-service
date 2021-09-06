package auth_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/auth"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/test"
	"github.com/codeready-toolchain/registration-service/test/fake"
	commonconfig "github.com/codeready-toolchain/toolchain-common/pkg/configuration"
	commontest "github.com/codeready-toolchain/toolchain-common/pkg/test"
	authsupport "github.com/codeready-toolchain/toolchain-common/pkg/test/auth"
	testconfig "github.com/codeready-toolchain/toolchain-common/pkg/test/config"
	"github.com/gofrs/uuid"
	"github.com/golang-jwt/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestKeyManagerSuite struct {
	test.UnitTestSuite
}

func TestRunKeyManagerSuite(t *testing.T) {
	suite.Run(t, &TestKeyManagerSuite{
		test.UnitTestSuite{},
	})
}

func (s *TestKeyManagerSuite) TestKeyFetching() {
	restore := commontest.SetEnvVarAndRestore(s.T(), commonconfig.WatchNamespaceEnvVar, commontest.HostOperatorNs)
	defer restore()

	// create test keys
	tokengenerator := authsupport.NewTokenManager()
	kid0 := uuid.Must(uuid.NewV4()).String()
	_, err := tokengenerator.AddPrivateKey(kid0)
	require.NoError(s.T(), err)
	kid1 := uuid.Must(uuid.NewV4()).String()
	_, err = tokengenerator.AddPrivateKey(kid1)
	require.NoError(s.T(), err)

	// create two test tokens, both valid
	username0 := uuid.Must(uuid.NewV4()).String()
	identity0 := &authsupport.Identity{
		ID:       uuid.Must(uuid.NewV4()),
		Username: username0,
	}
	email0 := identity0.Username + "@email.tld"
	jwt0, err := tokengenerator.GenerateSignedToken(*identity0, kid0, authsupport.WithEmailClaim(email0))
	require.NoError(s.T(), err)
	username1 := uuid.Must(uuid.NewV4()).String()
	identity1 := &authsupport.Identity{
		ID:       uuid.Must(uuid.NewV4()),
		Username: username1,
	}
	email1 := identity1.Username + "@email.tld"
	jwt1, err := tokengenerator.GenerateSignedToken(*identity1, kid1, authsupport.WithEmailClaim(email1))
	require.NoError(s.T(), err)

	// startup public key service
	keysEndpointURL := tokengenerator.NewKeyServer().URL

	// Set the config for testing mode and the key service url in the config, the handler may use this.
	s.OverrideApplicationDefault(testconfig.RegistrationService().
		Environment(configuration.DefaultEnvironment).
		Auth().AuthClientPublicKeysURL(keysEndpointURL))
	assert.False(s.T(), configuration.IsTestingMode(), "testing mode not set correctly to false")
	cfg := configuration.GetRegistrationServiceConfig()
	assert.Equal(s.T(), keysEndpointURL, cfg.Auth().AuthClientPublicKeysURL(), "key url not set correctly")

	s.Run("parse keys, valid response", func() {
		// Create KeyManager instance.
		keyManager, err := auth.NewKeyManager()
		require.NoError(s.T(), err)

		// check if the keys are parsed correctly
		_, err = keyManager.Key(kid0)
		require.NoError(s.T(), err)
		_, err = keyManager.Key(kid1)
		require.NoError(s.T(), err)
	})

	s.Run("parse keys, invalid response status code", func() {
		// setup http service serving the test keys
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, err := fmt.Fprintln(w, `{some: "invalid", "json"}`)
			assert.NoError(s.T(), err)
		}))
		defer ts.Close()

		// check if service runs
		_, err := http.Get(ts.URL)
		require.NoError(s.T(), err)

		// Set the config for testing mode, the handler may use this.
		s.OverrideApplicationDefault(testconfig.RegistrationService().
			Auth().AuthClientPublicKeysURL(ts.URL))
		cfg := configuration.GetRegistrationServiceConfig()
		assert.Equal(s.T(), cfg.Auth().AuthClientPublicKeysURL(), ts.URL, "key url not set correctly for testing")

		// Create KeyManager instance.
		_, err = auth.NewKeyManager()
		// this needs to fail with an error
		assert.EqualError(s.T(), err, "unable to obtain public keys from remote service")
	})

	s.Run("parse keys, invalid response", func() {
		// setup http service serving the test keys
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, err := fmt.Fprintln(w, `{some: "invalid", "json"}`)
			assert.NoError(s.T(), err)
		}))
		defer ts.Close()

		// check if service runs
		_, err := http.Get(ts.URL)
		require.NoError(s.T(), err)

		// Set the config for testing mode, the handler may use this.
		s.OverrideApplicationDefault(testconfig.RegistrationService().
			Auth().AuthClientPublicKeysURL(ts.URL))
		cfg := configuration.GetRegistrationServiceConfig()
		assert.Equal(s.T(), cfg.Auth().AuthClientPublicKeysURL(), ts.URL, "key url not set correctly for testing")

		// Create KeyManager instance.
		_, err = auth.NewKeyManager()
		// this needs to fail with an error
		assert.EqualError(s.T(), err, "invalid character 's' looking for beginning of object key string")
	})

	s.Run("parse keys, invalid url", func() {
		// Set the config for testing mode, the handler may use this.
		notAnURL := "not an url"
		s.OverrideApplicationDefault(testconfig.RegistrationService().
			Auth().AuthClientPublicKeysURL(notAnURL))
		cfg := configuration.GetRegistrationServiceConfig()
		assert.Equal(s.T(), cfg.Auth().AuthClientPublicKeysURL(), notAnURL, "key url not set correctly for testing")

		// Create KeyManager instance.
		_, err := auth.NewKeyManager()
		// this needs to fail with an error
		require.Error(s.T(), err)
		assert.Contains(s.T(), err.Error(), "not%20an%20url")
		assert.Contains(s.T(), err.Error(), ": unsupported protocol scheme")
	})

	s.Run("parse keys, server not reachable", func() {
		// Set the config for testing mode, the handler may use this.
		anURL := "http://www.google.com/"
		s.OverrideApplicationDefault(testconfig.RegistrationService().
			Auth().AuthClientPublicKeysURL(anURL))
		cfg := configuration.GetRegistrationServiceConfig()
		assert.Equal(s.T(), cfg.Auth().AuthClientPublicKeysURL(), anURL, "key url not set correctly for testing")

		// Create KeyManager instance.
		_, err := auth.NewKeyManager()
		// this needs to fail with an error
		assert.EqualError(s.T(), err, "invalid character '<' looking for beginning of value")
	})

	s.Run("validate with valid keys", func() {
		// Create KeyManager instance.
		s.OverrideApplicationDefault(testconfig.RegistrationService().
			Auth().AuthClientPublicKeysURL(keysEndpointURL))
		cfg := configuration.GetRegistrationServiceConfig()

		assert.Equal(s.T(), cfg.Auth().AuthClientPublicKeysURL(), keysEndpointURL, "key url not set correctly for testing")
		keyManager, err := auth.NewKeyManager()

		// check if the keys can be used to verify a JWT
		var statictests = []struct {
			name string
			jwt  string
			kid  string
		}{
			{"valid JWT 0", jwt0, kid0},
			{"valid JWT 1", jwt1, kid1},
		}
		for _, tt := range statictests {
			s.Run(tt.name, func() {
				_, err = jwt.Parse(tt.jwt, func(token *jwt.Token) (interface{}, error) {
					kid := token.Header["kid"]
					require.Equal(s.T(), tt.kid, kid)
					return keyManager.Key(kid.(string))
				})
				require.NoError(s.T(), err)
			})
		}
	})

	s.Run("validate with invalid keys", func() {
		// Create KeyManager instance.
		s.OverrideApplicationDefault(testconfig.RegistrationService().
			Auth().AuthClientPublicKeysURL(keysEndpointURL))
		cfg := configuration.GetRegistrationServiceConfig()

		assert.Equal(s.T(), cfg.Auth().AuthClientPublicKeysURL(), keysEndpointURL, "key url not set correctly for testing")
		keyManager, err := auth.NewKeyManager()

		// check if the keys can be used to verify a JWT
		var statictests = []struct {
			name string
			jwt  string
			kid  string
		}{
			{"valid JWT 0", jwt0, kid1},
			{"valid JWT 1", jwt1, kid0},
		}
		for _, tt := range statictests {
			s.Run(tt.name, func() {
				_, err = jwt.Parse(tt.jwt, func(token *jwt.Token) (interface{}, error) {
					kid := token.Header["kid"]
					require.NotEqual(s.T(), tt.kid, kid)
					return keyManager.Key(tt.kid)
				})
				assert.EqualError(s.T(), err, "crypto/rsa: verification error")
			})
		}
	})
}

func (s *TestKeyManagerSuite) TestE2EKeyFetching() {
	restore := commontest.SetEnvVarAndRestore(s.T(), commonconfig.WatchNamespaceEnvVar, commontest.HostOperatorNs)
	defer restore()

	s.Run("retrieve key for e2e-tests environment", func() {
		s.OverrideApplicationDefault(testconfig.RegistrationService().
			Environment("e2e-tests"))
		keyManager, err := auth.NewKeyManager()
		require.NoError(s.T(), err)
		keys := authsupport.GetE2ETestPublicKey()

		for _, key := range keys {
			// check if the keys are parsed correctly.
			_, err = keyManager.Key(key.KeyID)
			require.NoError(s.T(), err)
		}
	})

	checkE2EKeysNotFound := func() {
		keyManager, err := auth.NewKeyManager()
		require.NoError(s.T(), err)
		keys := authsupport.GetE2ETestPublicKey()

		for _, key := range keys {
			// check that key is not found as the environment
			// is not set to e2e-tests
			_, err = keyManager.Key(key.KeyID)
			assert.EqualError(s.T(), err, "unknown kid")
		}
	}

	s.Run("fail to retrieve e2e keys for default environment", func() {
		s.DefaultConfig()
		fake.MockKeycloakCertsCall(s.T())

		checkE2EKeysNotFound()
	})

	s.Run("fail to retrieve e2e keys for prod environment", func() {
		s.OverrideApplicationDefault(testconfig.RegistrationService().
			Environment("prod"))
		defer s.DefaultConfig()
		fake.MockKeycloakCertsCall(s.T())

		checkE2EKeysNotFound()
	})

	s.Run("fail to retrieve e2e keys if environment is unexpected value", func() {
		s.OverrideApplicationDefault(testconfig.RegistrationService().
			Environment("unexpected"))
		defer s.DefaultConfig()
		fake.MockKeycloakCertsCall(s.T())

		checkE2EKeysNotFound()
	})
}
