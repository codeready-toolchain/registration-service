package auth_test

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/auth"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	testutils "github.com/codeready-toolchain/registration-service/test"
	jwt "github.com/dgrijalva/jwt-go"
	uuid "github.com/satori/go.uuid"

	//"github.com/dgrijalva/jwt-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeyManager(t *testing.T) {
	// Create logger and registry.
	logger := log.New(os.Stderr, "", 0)
	configRegistry := configuration.CreateEmptyRegistry()

	// Set the config for testing mode, the handler may use this.
	configRegistry.GetViperInstance().Set("testingmode", true)
	assert.True(t, configRegistry.IsTestingMode(), "testing mode not set correctly to true")

	t.Run("missing logger", func(t *testing.T) {
		_, err := auth.NewKeyManager(nil, configRegistry)
		require.Error(t, err)
	})

	t.Run("missing config", func(t *testing.T) {
		_, err := auth.NewKeyManager(logger, nil)
		require.Error(t, err)
	})

	t.Run("missing logger and config", func(t *testing.T) {
		_, err := auth.NewKeyManager(nil, nil)
		require.Error(t, err)
	})
}

func TestKeyFetching(t *testing.T) {
	// Create logger and registry.
	logger := log.New(os.Stderr, "", 0)
	configRegistry := configuration.CreateEmptyRegistry()

	// create test keys
	tokengenerator := testutils.NewTokenManager()
	kid0 := uuid.NewV4().String()
	_, err := tokengenerator.AddPrivateKey(kid0)
	require.NoError(t, err)
	kid1 := uuid.NewV4().String()
	_, err = tokengenerator.AddPrivateKey(kid1)
	require.NoError(t, err)

	// create two test tokens, both valid
	username0 := uuid.NewV4().String()
	identity0 := &testutils.Identity{
		ID:       uuid.NewV4(),
		Username: username0,
	}
	email0 := identity0.Username + "@email.tld"
	jwt0, err := tokengenerator.GenerateSignedToken(*identity0, kid0, testutils.WithEmailClaim(email0))
	require.NoError(t, err)
	username1 := uuid.NewV4().String()
	identity1 := &testutils.Identity{
		ID:       uuid.NewV4(),
		Username: username1,
	}
	email1 := identity1.Username + "@email.tld"
	jwt1, err := tokengenerator.GenerateSignedToken(*identity1, kid1, testutils.WithEmailClaim(email1))
	require.NoError(t, err)

	// startup public key service
	keysEndpointURL := tokengenerator.NewKeyServer().URL

	// Set the config for testing mode, the handler may use this.
	configRegistry.GetViperInstance().Set("testingmode", false)
	assert.False(t, configRegistry.IsTestingMode(), "testing mode not set correctly to false")
	// set the key service url in the config
	configRegistry.GetViperInstance().Set("auth_client.public_keys_url", keysEndpointURL)
	assert.Equal(t, keysEndpointURL, configRegistry.GetAuthClientPublicKeysURL(), "key url not set correctly")

	t.Run("parse keys, valid response", func(t *testing.T) {
		// Create KeyManager instance.
		keyManager, err := auth.NewKeyManager(logger, configRegistry)
		require.NoError(t, err)

		// check if the keys are parsed correctly
		_, err = keyManager.Key(kid0)
		require.NoError(t, err)
		_, err = keyManager.Key(kid1)
		require.NoError(t, err)
	})

	t.Run("parse keys, invalid response status code", func(t *testing.T) {
		// setup http service serving the test keys
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, `{some: "invalid", "json"}`)
		}))
		defer ts.Close()

		// check if service runs
		_, err := http.Get(ts.URL)
		require.NoError(t, err)

		// Set the config for testing mode, the handler may use this.
		configRegistry.GetViperInstance().Set("auth_client.public_keys_url", ts.URL)
		assert.Equal(t, configRegistry.GetAuthClientPublicKeysURL(), ts.URL, "key url not set correctly for testing")

		// Create KeyManager instance.
		_, err = auth.NewKeyManager(logger, configRegistry)
		// this needs to fail with an error
		require.Error(t, err)
	})

	t.Run("parse keys, invalid response", func(t *testing.T) {
		// setup http service serving the test keys
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{some: "invalid", "json"}`)
		}))
		defer ts.Close()

		// check if service runs
		_, err := http.Get(ts.URL)
		require.NoError(t, err)

		// Set the config for testing mode, the handler may use this.
		configRegistry.GetViperInstance().Set("auth_client.public_keys_url", ts.URL)
		assert.Equal(t, configRegistry.GetAuthClientPublicKeysURL(), ts.URL, "key url not set correctly for testing")

		// Create KeyManager instance.
		_, err = auth.NewKeyManager(logger, configRegistry)
		// this needs to fail with an error
		require.Error(t, err)
	})

	t.Run("parse keys, invalid url", func(t *testing.T) {
		// Set the config for testing mode, the handler may use this.
		notAnURL := "not an url"
		configRegistry.GetViperInstance().Set("auth_client.public_keys_url", notAnURL)
		assert.Equal(t, configRegistry.GetAuthClientPublicKeysURL(), notAnURL, "key url not set correctly for testing")

		// Create KeyManager instance.
		_, err := auth.NewKeyManager(logger, configRegistry)
		// this needs to fail with an error
		require.Error(t, err)
	})

	t.Run("parse keys, server not reachable", func(t *testing.T) {
		// Set the config for testing mode, the handler may use this.
		anURL := "http://www.google.com/"
		configRegistry.GetViperInstance().Set("auth_client.public_keys_url", anURL)
		assert.Equal(t, configRegistry.GetAuthClientPublicKeysURL(), anURL, "key url not set correctly for testing")

		// Create KeyManager instance.
		_, err := auth.NewKeyManager(logger, configRegistry)
		// this needs to fail with an error
		require.Error(t, err)
	})

	t.Run("validate with valid keys", func(t *testing.T) {
		// Create KeyManager instance.
		configRegistry.GetViperInstance().Set("auth_client.public_keys_url", keysEndpointURL)
		assert.Equal(t, configRegistry.GetAuthClientPublicKeysURL(), keysEndpointURL, "key url not set correctly for testing")
		keyManager, err := auth.NewKeyManager(logger, configRegistry)

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
			t.Run(tt.name, func(t *testing.T) {
				_, err = jwt.Parse(tt.jwt, func(token *jwt.Token) (interface{}, error) {
					kid := token.Header["kid"]
					require.Equal(t, tt.kid, kid)
					return keyManager.Key(kid.(string))
				})
				require.NoError(t, err)
			})
		}
	})

	t.Run("validate with invalid keys", func(t *testing.T) {
		// Create KeyManager instance.
		configRegistry.GetViperInstance().Set("auth_client.public_keys_url", keysEndpointURL)
		assert.Equal(t, configRegistry.GetAuthClientPublicKeysURL(), keysEndpointURL, "key url not set correctly for testing")
		keyManager, err := auth.NewKeyManager(logger, configRegistry)

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
			t.Run(tt.name, func(t *testing.T) {
				_, err = jwt.Parse(tt.jwt, func(token *jwt.Token) (interface{}, error) {
					kid := token.Header["kid"]
					require.NotEqual(t, tt.kid, kid)
					return keyManager.Key(tt.kid)
				})
				require.Error(t, err)
				require.EqualError(t, err, "crypto/rsa: verification error")
			})
		}
	})
}
