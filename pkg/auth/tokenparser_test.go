package auth_test

import (
	"strings"
	"testing"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/auth"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/test"
	authsupport "github.com/codeready-toolchain/toolchain-common/pkg/test/auth"

	"github.com/dgrijalva/jwt-go"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestTokenParserSuite struct {
	test.UnitTestSuite
}

func TestRunTokenParserSuite(t *testing.T) {
	suite.Run(t, &TestTokenParserSuite{test.UnitTestSuite{}})
}

func (s *TestTokenParserSuite) TestTokenParser() {
	configRegistry := configuration.CreateEmptyRegistry()

	// create test keys
	tokengenerator := authsupport.NewTokenManager()
	kid0 := uuid.NewV4().String()
	_, err := tokengenerator.AddPrivateKey(kid0)
	require.NoError(s.T(), err)
	kid1 := uuid.NewV4().String()
	_, err = tokengenerator.AddPrivateKey(kid1)
	require.NoError(s.T(), err)

	// startup public key service
	keysEndpointURL := tokengenerator.NewKeyServer().URL

	// set the config for testing mode, the handler may use this.
	configRegistry.GetViperInstance().Set("environment", configuration.UnitTestsEnvironment)
	assert.True(s.T(), configRegistry.IsTestingMode(), "testing mode not set correctly to true")
	// set the key service url in the config
	configRegistry.GetViperInstance().Set("auth_client.public_keys_url", keysEndpointURL)
	assert.Equal(s.T(), keysEndpointURL, configRegistry.GetAuthClientPublicKeysURL(), "key url not set correctly")

	// create KeyManager instance.
	keyManager, err := auth.NewKeyManager(configRegistry)
	require.NoError(s.T(), err)

	// create TokenParser instance.
	tokenParser, err := auth.NewTokenParser(keyManager)
	require.NoError(s.T(), err)

	s.Run("invalid arguments to new", func() {
		_, err = auth.NewTokenParser(nil)
		require.Error(s.T(), err)
		require.Equal(s.T(), "no keyManager given when creating TokenParser", err.Error())
	})

	s.Run("parse valid tokens", func() {
		// create two test tokens, both valid
		username0 := uuid.NewV4().String()
		identity0 := &authsupport.Identity{
			ID:       uuid.NewV4(),
			Username: username0,
		}
		email0 := identity0.Username + "@email.tld"
		jwt0, err := tokengenerator.GenerateSignedToken(*identity0, kid0, authsupport.WithEmailClaim(email0))
		require.NoError(s.T(), err)
		username1 := uuid.NewV4().String()
		identity1 := &authsupport.Identity{
			ID:       uuid.NewV4(),
			Username: username1,
		}
		email1 := identity1.Username + "@email.tld"
		jwt1, err := tokengenerator.GenerateSignedToken(*identity1, kid1, authsupport.WithEmailClaim(email1))
		require.NoError(s.T(), err)

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
			s.Run(tt.name, func() {
				claims, err := tokenParser.FromString(tt.jwt)
				require.NoError(s.T(), err)
				require.Equal(s.T(), tt.username, claims.Username)
				require.Equal(s.T(), tt.email, claims.Email)
			})
		}
	})

	s.Run("parse invalid token", func() {
		// create invalid test token (wrong set of claims, no email), signed with key1
		username_invalid := uuid.NewV4().String()
		identity_invalid := &authsupport.Identity{
			ID:       uuid.NewV4(),
			Username: username_invalid,
		}
		jwt_invalid, err := tokengenerator.GenerateSignedToken(*identity_invalid, kid1)
		require.NoError(s.T(), err)

		_, err = tokenParser.FromString(jwt_invalid)
		require.Error(s.T(), err)
		require.EqualError(s.T(), err, "token does not comply to expected claims: email missing")
	})

	s.Run("token signed by unknown key", func() {
		// new key
		kidX := uuid.NewV4().String()
		_, err := tokengenerator.AddPrivateKey(kidX)
		require.NoError(s.T(), err)
		// generate valid token
		usernameX := uuid.NewV4().String()
		identityX := &authsupport.Identity{
			ID:       uuid.NewV4(),
			Username: usernameX,
		}
		emailX := identityX.Username + "@email.tld"
		jwtX, err := tokengenerator.GenerateSignedToken(*identityX, kidX, authsupport.WithEmailClaim(emailX))
		require.NoError(s.T(), err)
		// remove key from known keys
		tokengenerator.RemovePrivateKey(kidX)
		// validate token
		_, err = tokenParser.FromString(jwtX)
		require.Error(s.T(), err)
		require.EqualError(s.T(), err, "unknown kid")
	})

	s.Run("no KID header in token", func() {
		username0 := uuid.NewV4().String()
		identity0 := &authsupport.Identity{
			ID:       uuid.NewV4(),
			Username: username0,
		}
		email0 := identity0.Username + "@email.tld"
		// generate non-serialized token
		jwt0 := tokengenerator.GenerateToken(*identity0, kid0, authsupport.WithEmailClaim(email0))
		delete(jwt0.Header, "kid")
		// serialize
		jwt0string, err := tokengenerator.SignToken(jwt0, kid0)
		require.NoError(s.T(), err)
		// validate token
		_, err = tokenParser.FromString(jwt0string)
		require.Error(s.T(), err)
		require.EqualError(s.T(), err, "no key id given in the token")
	})

	s.Run("missing claim: preferred_username", func() {
		username0 := uuid.NewV4().String()
		identity0 := &authsupport.Identity{
			ID:       uuid.NewV4(),
			Username: username0,
		}
		email0 := identity0.Username + "@email.tld"
		// generate non-serialized token
		jwt0 := tokengenerator.GenerateToken(*identity0, kid0, authsupport.WithEmailClaim(email0))
		// delete preferred_username
		delete(jwt0.Claims.(jwt.MapClaims), "preferred_username")
		// serialize
		jwt0string, err := tokengenerator.SignToken(jwt0, kid0)
		require.NoError(s.T(), err)
		// validate token
		_, err = tokenParser.FromString(jwt0string)
		require.Error(s.T(), err)
		require.EqualError(s.T(), err, "token does not comply to expected claims: username missing")
	})

	s.Run("missing claim: email", func() {
		username0 := uuid.NewV4().String()
		identity0 := &authsupport.Identity{
			ID:       uuid.NewV4(),
			Username: username0,
		}
		// generate non-serialized token
		jwt0 := tokengenerator.GenerateToken(*identity0, kid0)
		// serialize
		jwt0string, err := tokengenerator.SignToken(jwt0, kid0)
		require.NoError(s.T(), err)
		// validate token
		_, err = tokenParser.FromString(jwt0string)
		require.Error(s.T(), err)
		require.EqualError(s.T(), err, "token does not comply to expected claims: email missing")
	})

	s.Run("missing claim: sub", func() {
		username0 := uuid.NewV4().String()
		identity0 := &authsupport.Identity{
			ID:       uuid.NewV4(),
			Username: username0,
		}
		email0 := identity0.Username + "@email.tld"
		// generate non-serialized token
		jwt0 := tokengenerator.GenerateToken(*identity0, kid0, authsupport.WithEmailClaim(email0))
		// delete preferred_username
		delete(jwt0.Claims.(jwt.MapClaims), "sub")
		// serialize
		jwt0string, err := tokengenerator.SignToken(jwt0, kid0)
		require.NoError(s.T(), err)
		// validate token
		_, err = tokenParser.FromString(jwt0string)
		require.Error(s.T(), err)
		require.EqualError(s.T(), err, "token does not comply to expected claims: subject missing")
	})

	s.Run("signature is good but token expired", func() {
		username0 := uuid.NewV4().String()
		identity0 := &authsupport.Identity{
			ID:       uuid.NewV4(),
			Username: username0,
		}
		email0 := identity0.Username + "@email.tld"
		// generate non-serialized token
		jwt0 := tokengenerator.GenerateToken(*identity0, kid0, authsupport.WithEmailClaim(email0))
		// manipulate expiry
		tDiff := -60 * time.Second
		jwt0.Claims.(jwt.MapClaims)["exp"] = time.Now().UTC().Add(tDiff).Unix()
		// serialize
		jwt0string, err := tokengenerator.SignToken(jwt0, kid0)
		require.NoError(s.T(), err)
		// validate token
		_, err = tokenParser.FromString(jwt0string)
		require.Error(s.T(), err)
		require.True(s.T(), strings.HasPrefix(err.Error(), "token is expired by "))
	})

	s.Run("signature is good but token not valid yet", func() {
		username0 := uuid.NewV4().String()
		identity0 := &authsupport.Identity{
			ID:       uuid.NewV4(),
			Username: username0,
		}
		email0 := identity0.Username + "@email.tld"
		// generate non-serialized token
		jwt0 := tokengenerator.GenerateToken(*identity0, kid0, authsupport.WithEmailClaim(email0))
		// manipulate expiry
		tDiff := 60 * time.Second
		jwt0.Claims.(jwt.MapClaims)["nbf"] = time.Now().UTC().Add(tDiff).Unix()
		// serialize
		jwt0string, err := tokengenerator.SignToken(jwt0, kid0)
		require.NoError(s.T(), err)
		// validate token
		_, err = tokenParser.FromString(jwt0string)
		require.Error(s.T(), err)
		require.EqualError(s.T(), err, "token is not valid yet")
	})

	s.Run("token signed by known key but the signature is invalid", func() {
		username0 := uuid.NewV4().String()
		identity0 := &authsupport.Identity{
			ID:       uuid.NewV4(),
			Username: username0,
		}
		email0 := identity0.Username + "@email.tld"
		// generate non-serialized token
		jwt0 := tokengenerator.GenerateToken(*identity0, kid0, authsupport.WithEmailClaim(email0))
		// serialize
		jwt0string, err := tokengenerator.SignToken(jwt0, kid0)
		require.NoError(s.T(), err)
		// replace signature with garbage
		str := strings.Split(jwt0string, ".")
		require.Len(s.T(), str, 3)
		str[2] = uuid.NewV4().String()
		jwt0string = strings.Join(str, ".")
		// validate token
		_, err = tokenParser.FromString(jwt0string)
		require.Error(s.T(), err)
		require.EqualError(s.T(), err, "crypto/rsa: verification error")
	})
}
