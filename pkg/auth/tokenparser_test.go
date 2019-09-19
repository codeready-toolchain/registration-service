package auth_test

import (
	"log"
	"os"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/auth"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	testutils "github.com/codeready-toolchain/registration-service/test"
	uuid "github.com/satori/go.uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenParser(t *testing.T) {
	// create logger and registry.
	logger := log.New(os.Stderr, "", 0)
	configRegistry := configuration.CreateEmptyRegistry()

	// create test keys
	tokengenerator := testutils.NewTokenGenerator()
	kid0 := uuid.NewV4().String()
	_, err := tokengenerator.CreateKey(kid0)
	require.NoError(t, err)
	kid1 := uuid.NewV4().String()
	_, err = tokengenerator.CreateKey(kid1)
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

	// create invalid test token (wrong set of claims, no email), signed with key1
	username_invalid := uuid.NewV4().String()
	identity_invalid := &testutils.Identity{
		ID:       uuid.NewV4(),
		Username: username_invalid,
	}
	jwt_invalid, err := tokengenerator.GenerateSignedToken(*identity_invalid, kid1)
	require.NoError(t, err)

	// startup public key service
	keysEndpointURL := tokengenerator.GetKeyService()

	// set the config for testing mode, the handler may use this.
	configRegistry.GetViperInstance().Set("testingmode", true)
	assert.True(t, configRegistry.IsTestingMode(), "testing mode not set correctly to true")
	// set the key service url in the config
	configRegistry.GetViperInstance().Set("auth_client.public_keys_url", keysEndpointURL)
	assert.Equal(t, keysEndpointURL, configRegistry.GetAuthClientPublicKeysURL(), "key url not set correctly")

	// create KeyManager instance.
	keyManager, err := auth.NewKeyManager(logger, configRegistry)
	require.NoError(t, err)

	// create TokenParser instance.
	tokenParser, err := auth.NewTokenParser(logger, configRegistry, keyManager)
	require.NoError(t, err)

	t.Run("invalid arguments to new", func(t *testing.T) {
		_, err := auth.NewTokenParser(nil, configRegistry, keyManager)
		require.Error(t, err)
		_, err = auth.NewTokenParser(logger, nil, keyManager)
		require.Error(t, err)
		_, err = auth.NewTokenParser(logger, configRegistry, nil)
		require.Error(t, err)
	})

	t.Run("parse valid tokens", func(t *testing.T) {
		// check if the keys can be used to verify a JWT
		var statictests = []struct {
			name     string
			jwt      string
			username string
			email    string
		}{
			{"valid JWT 0", jwt0, identity0.Username, email0},
			{"valid JWT 1", jwt1, identity1.Username, email1},
		}
		for _, tt := range statictests {
			t.Run(tt.name, func(t *testing.T) {
				claims, err := tokenParser.FromString(tt.jwt)
				require.NoError(t, err)
				require.Equal(t, tt.username, claims.Username)
				require.Equal(t, tt.email, claims.Email)
			})
		}
	})

	t.Run("parse invalid token", func(t *testing.T) {
		_, err := tokenParser.FromString(jwt_invalid)
		require.Error(t, err)
		require.EqualError(t, err, "token does not comply to expected claims: email missing")
	})
}
