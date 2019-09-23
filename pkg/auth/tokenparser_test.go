package auth_test

import (
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/auth"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	testutils "github.com/codeready-toolchain/registration-service/test"
	"github.com/dgrijalva/jwt-go"
	uuid "github.com/satori/go.uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenParser(t *testing.T) {
	// create logger and registry.
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

	// startup public key service
	keysEndpointURL := tokengenerator.NewKeyServer().URL

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
		require.Equal(t, "no logger given when creating TokenParser", err.Error())
		_, err = auth.NewTokenParser(logger, nil, keyManager)
		require.Error(t, err)
		require.Equal(t, "no config given when creating TokenParser", err.Error())
		_, err = auth.NewTokenParser(logger, configRegistry, nil)
		require.Error(t, err)
		require.Equal(t, "no keyManager given when creating TokenParser", err.Error())
	})

	t.Run("parse valid tokens", func(t *testing.T) {
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
		// create invalid test token (wrong set of claims, no email), signed with key1
		username_invalid := uuid.NewV4().String()
		identity_invalid := &testutils.Identity{
			ID:       uuid.NewV4(),
			Username: username_invalid,
		}
		jwt_invalid, err := tokengenerator.GenerateSignedToken(*identity_invalid, kid1)
		require.NoError(t, err)

		_, err = tokenParser.FromString(jwt_invalid)
		require.Error(t, err)
		require.EqualError(t, err, "token does not comply to expected claims: email missing")
	})

	t.Run("token signed by unknown key", func(t *testing.T) {
		// new key
		kidX := uuid.NewV4().String()
		_, err := tokengenerator.AddPrivateKey(kidX)
		require.NoError(t, err)
		// generate valid token
		usernameX := uuid.NewV4().String()
		identityX := &testutils.Identity{
			ID:       uuid.NewV4(),
			Username: usernameX,
		}
		emailX := identityX.Username + "@email.tld"
		jwtX, err := tokengenerator.GenerateSignedToken(*identityX, kidX, testutils.WithEmailClaim(emailX))
		require.NoError(t, err)
		// remove key from known keys
		tokengenerator.RemovePrivateKey(kidX)
		// validate token
		_, err = tokenParser.FromString(jwtX)
		require.Error(t, err)
		require.EqualError(t, err, "unknown kid")
	})

	t.Run("no KID header in token", func(t *testing.T) {
		username0 := uuid.NewV4().String()
		identity0 := &testutils.Identity{
			ID:       uuid.NewV4(),
			Username: username0,
		}
		email0 := identity0.Username + "@email.tld"
		// generate non-serialized token
		jwt0 := tokengenerator.GenerateToken(*identity0, kid0, testutils.WithEmailClaim(email0))
		delete(jwt0.Header, "kid")
		// serialize
		jwt0string, err := tokengenerator.SignToken(jwt0, kid0)
		require.NoError(t, err)
		// validate token
		_, err = tokenParser.FromString(jwt0string)
		require.Error(t, err)
		require.EqualError(t, err, "no key id given in the token")
	})

	t.Run("missing claim: preferred_username", func(t *testing.T) {
		username0 := uuid.NewV4().String()
		identity0 := &testutils.Identity{
			ID:       uuid.NewV4(),
			Username: username0,
		}
		email0 := identity0.Username + "@email.tld"
		// generate non-serialized token
		jwt0 := tokengenerator.GenerateToken(*identity0, kid0, testutils.WithEmailClaim(email0))
		// delete preferred_username
		delete(jwt0.Claims.(jwt.MapClaims), "preferred_username")
		// serialize
		jwt0string, err := tokengenerator.SignToken(jwt0, kid0)
		require.NoError(t, err)
		// validate token
		_, err = tokenParser.FromString(jwt0string)
		require.Error(t, err)
		require.EqualError(t, err, "token does not comply to expected claims: username missing")
	})

	t.Run("missing claim: email", func(t *testing.T) {
		username0 := uuid.NewV4().String()
		identity0 := &testutils.Identity{
			ID:       uuid.NewV4(),
			Username: username0,
		}
		// generate non-serialized token
		jwt0 := tokengenerator.GenerateToken(*identity0, kid0)
		// serialize
		jwt0string, err := tokengenerator.SignToken(jwt0, kid0)
		require.NoError(t, err)
		// validate token
		_, err = tokenParser.FromString(jwt0string)
		require.Error(t, err)
		require.EqualError(t, err, "token does not comply to expected claims: email missing")
	})

	t.Run("signature is good but token expired", func(t *testing.T) {
		username0 := uuid.NewV4().String()
		identity0 := &testutils.Identity{
			ID:       uuid.NewV4(),
			Username: username0,
		}
		email0 := identity0.Username + "@email.tld"
		// generate non-serialized token
		jwt0 := tokengenerator.GenerateToken(*identity0, kid0, testutils.WithEmailClaim(email0))
		// manipulate expiry
		tDiff := -60 * time.Second
		jwt0.Claims.(jwt.MapClaims)["exp"] = time.Now().UTC().Add(tDiff).Unix()
		// serialize
		jwt0string, err := tokengenerator.SignToken(jwt0, kid0)
		require.NoError(t, err)
		// validate token
		_, err = tokenParser.FromString(jwt0string)
		require.Error(t, err)
		require.EqualError(t, err, "token is expired by 1m0s")
	})

	t.Run("signature is good but token not valid yet", func(t *testing.T) {
		username0 := uuid.NewV4().String()
		identity0 := &testutils.Identity{
			ID:       uuid.NewV4(),
			Username: username0,
		}
		email0 := identity0.Username + "@email.tld"
		// generate non-serialized token
		jwt0 := tokengenerator.GenerateToken(*identity0, kid0, testutils.WithEmailClaim(email0))
		// manipulate expiry
		tDiff := 60 * time.Second
		jwt0.Claims.(jwt.MapClaims)["nbf"] = time.Now().UTC().Add(tDiff).Unix()
		// serialize
		jwt0string, err := tokengenerator.SignToken(jwt0, kid0)
		require.NoError(t, err)
		// validate token
		_, err = tokenParser.FromString(jwt0string)
		require.Error(t, err)
		require.EqualError(t, err, "token is not valid yet")
	})

	t.Run("token signed by known key but the signature is invalid", func(t *testing.T) {
		username0 := uuid.NewV4().String()
		identity0 := &testutils.Identity{
			ID:       uuid.NewV4(),
			Username: username0,
		}
		email0 := identity0.Username + "@email.tld"
		// generate non-serialized token
		jwt0 := tokengenerator.GenerateToken(*identity0, kid0, testutils.WithEmailClaim(email0))
		// serialize
		jwt0string, err := tokengenerator.SignToken(jwt0, kid0)
		require.NoError(t, err)
		// replace signature with garbage
		s := strings.Split(jwt0string, ".")
		require.Len(t, s, 3)
		s[2] = uuid.NewV4().String()
		jwt0string = strings.Join(s, ".")
		// validate token
		_, err = tokenParser.FromString(jwt0string)
		require.Error(t, err)
		require.EqualError(t, err, "crypto/rsa: verification error")
	})
}
