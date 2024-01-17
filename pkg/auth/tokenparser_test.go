package auth_test

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/codeready-toolchain/registration-service/pkg/auth"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/test"
	commonconfig "github.com/codeready-toolchain/toolchain-common/pkg/configuration"
	commontest "github.com/codeready-toolchain/toolchain-common/pkg/test"
	authsupport "github.com/codeready-toolchain/toolchain-common/pkg/test/auth"
	testconfig "github.com/codeready-toolchain/toolchain-common/pkg/test/config"

	"github.com/gofrs/uuid"
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

	// startup public key service
	keysEndpointURL := tokengenerator.NewKeyServer().URL

	// set the config for testing mode, the handler may use this.
	// set the key service url in the config
	s.OverrideApplicationDefault(testconfig.RegistrationService().
		Environment(configuration.UnitTestsEnvironment).
		Auth().AuthClientPublicKeysURL(keysEndpointURL))
	cfg := configuration.GetRegistrationServiceConfig()

	assert.True(s.T(), configuration.IsTestingMode(), "testing mode not set correctly to true")
	assert.Equal(s.T(), keysEndpointURL, cfg.Auth().AuthClientPublicKeysURL(), "key url not set correctly")

	// create KeyManager instance.
	keyManager, err := auth.NewKeyManager()
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
				require.Equal(s.T(), tt.username, claims.PreferredUsername)
				require.Equal(s.T(), tt.email, claims.Email)
			})
		}
	})

	s.Run("parse invalid token", func() {
		// create invalid test token (wrong set of claims, no email), signed with key1
		invalidUsername := uuid.Must(uuid.NewV4()).String()
		invalidIdentity := &authsupport.Identity{
			ID:       uuid.Must(uuid.NewV4()),
			Username: invalidUsername,
		}
		invalidJWT, err := tokengenerator.GenerateSignedToken(*invalidIdentity, kid1)
		require.NoError(s.T(), err)

		_, err = tokenParser.FromString(invalidJWT)
		require.Error(s.T(), err)
		require.EqualError(s.T(), err, "token does not comply to expected claims: email missing")
	})

	s.Run("unexpected signing method", func() {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"foo": "bar",
			"nbf": time.Date(2015, 10, 10, 12, 0, 0, 0, time.UTC).Unix(),
		})
		// serialize
		jwt0string, err := token.SignedString([]byte("secret"))
		require.NoError(s.T(), err)
		// validate token
		_, err = tokenParser.FromString(jwt0string)
		require.Error(s.T(), err)
		require.EqualError(s.T(), err, "token is unverifiable: error while executing keyfunc: unexpected signing method: HS256")
	})

	s.Run("token signed by unknown key", func() {
		// new key
		kidX := uuid.Must(uuid.NewV4()).String()
		_, err := tokengenerator.AddPrivateKey(kidX)
		require.NoError(s.T(), err)
		// generate valid token
		usernameX := uuid.Must(uuid.NewV4()).String()
		identityX := &authsupport.Identity{
			ID:       uuid.Must(uuid.NewV4()),
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
		require.EqualError(s.T(), err, "token is unverifiable: error while executing keyfunc: unknown kid")
	})

	s.Run("no KID header in token", func() {
		username0 := uuid.Must(uuid.NewV4()).String()
		identity0 := &authsupport.Identity{
			ID:       uuid.Must(uuid.NewV4()),
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
		require.EqualError(s.T(), err, "token is unverifiable: error while executing keyfunc: no key id given in the token")
	})

	s.Run("missing claim: preferred_username", func() {
		username0 := uuid.Must(uuid.NewV4()).String()
		identity0 := &authsupport.Identity{
			ID:       uuid.Must(uuid.NewV4()),
			Username: username0,
		}
		email0 := identity0.Username + "@email.tld"
		// generate non-serialized token
		jwt0 := tokengenerator.GenerateToken(*identity0, kid0, authsupport.WithEmailClaim(email0))
		// delete preferred_username
		jwt0.Claims.(*authsupport.MyClaims).PreferredUsername = ""
		// serialize
		jwt0string, err := tokengenerator.SignToken(jwt0, kid0)
		require.NoError(s.T(), err)
		// validate token
		_, err = tokenParser.FromString(jwt0string)
		require.Error(s.T(), err)
		require.EqualError(s.T(), err, "token does not comply to expected claims: username missing")
	})

	s.Run("missing claim: email", func() {
		username0 := uuid.Must(uuid.NewV4()).String()
		identity0 := &authsupport.Identity{
			ID:       uuid.Must(uuid.NewV4()),
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
		username0 := uuid.Must(uuid.NewV4()).String()
		identity0 := &authsupport.Identity{
			ID:       uuid.Must(uuid.NewV4()),
			Username: username0,
		}
		email0 := identity0.Username + "@email.tld"
		// generate non-serialized token
		jwt0 := tokengenerator.GenerateToken(*identity0, kid0, authsupport.WithEmailClaim(email0), authsupport.WithSubClaim(""))

		// serialize
		jwt0string, err := tokengenerator.SignToken(jwt0, kid0)
		require.NoError(s.T(), err)
		// validate token
		_, err = tokenParser.FromString(jwt0string)
		require.Error(s.T(), err)
		require.EqualError(s.T(), err, "token does not comply to expected claims: subject missing")
	})

	s.Run("signature is good but token expired", func() {
		username0 := uuid.Must(uuid.NewV4()).String()
		identity0 := &authsupport.Identity{
			ID:       uuid.Must(uuid.NewV4()),
			Username: username0,
		}
		email0 := identity0.Username + "@email.tld"
		expTime := time.Now().Add(-60 * time.Second)
		expClaim := authsupport.WithExpClaim(expTime)
		// generate non-serialized token
		jwt0 := tokengenerator.GenerateToken(*identity0, kid0, authsupport.WithEmailClaim(email0), expClaim)

		// serialize
		jwt0string, err := tokengenerator.SignToken(jwt0, kid0)
		require.NoError(s.T(), err)
		// validate token
		_, err = tokenParser.FromString(jwt0string)
		require.Error(s.T(), err)
		require.EqualError(s.T(), err, "token has invalid claims: token is expired")
	})

	s.Run("signature is good but token not valid yet", func() {
		username0 := uuid.Must(uuid.NewV4()).String()
		identity0 := &authsupport.Identity{
			ID:       uuid.Must(uuid.NewV4()),
			Username: username0,
		}
		email0 := identity0.Username + "@email.tld"
		nbfTime := time.Now().Add(60 * time.Second)
		nbfClaim := authsupport.WithNotBeforeClaim(nbfTime)
		// generate non-serialized token
		jwt0 := tokengenerator.GenerateToken(*identity0, kid0, authsupport.WithEmailClaim(email0), nbfClaim)

		// serialize
		jwt0string, err := tokengenerator.SignToken(jwt0, kid0)
		require.NoError(s.T(), err)
		// validate token
		_, err = tokenParser.FromString(jwt0string)
		require.Error(s.T(), err)
		require.EqualError(s.T(), err, "token has invalid claims: token is not valid yet")
	})

	s.Run("signature is good and token expiration is within leeway", func() {
		username0 := uuid.Must(uuid.NewV4()).String()
		identity0 := &authsupport.Identity{
			ID:       uuid.Must(uuid.NewV4()),
			Username: username0,
		}
		email0 := identity0.Username + "@email.tld"
		expTime := time.Now().Add(-1 * time.Second)
		expClaim := authsupport.WithExpClaim(expTime)
		// generate non-serialized token
		jwt0 := tokengenerator.GenerateToken(*identity0, kid0, authsupport.WithEmailClaim(email0), expClaim)

		// serialize
		jwt0string, err := tokengenerator.SignToken(jwt0, kid0)
		require.NoError(s.T(), err)
		// validate token
		_, err = tokenParser.FromString(jwt0string)
		require.NoError(s.T(), err)
	})

	s.Run("token signed by known key but the signature is invalid", func() {
		username0 := uuid.Must(uuid.NewV4()).String()
		identity0 := &authsupport.Identity{
			ID:       uuid.Must(uuid.NewV4()),
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
		str[2] = uuid.Must(uuid.NewV4()).String()
		jwt0string = strings.Join(str, ".")
		// validate token
		_, err = tokenParser.FromString(jwt0string)
		require.Error(s.T(), err)
		require.EqualError(s.T(), err, "token signature is invalid: crypto/rsa: verification error")
	})

	s.Run("parse valid token with original_sub claim", func() {
		// create a test token with an original_sub claim
		username0 := uuid.Must(uuid.NewV4()).String()
		identity0 := &authsupport.Identity{
			ID:       uuid.Must(uuid.NewV4()),
			Username: username0,
		}
		email0 := identity0.Username + "@email.tld"

		originalSubClaim := func(token *jwt.Token) {
			token.Claims.(*authsupport.MyClaims).OriginalSub = "OriginalSubValue:1234-ABCD"
		}

		jwt0, err := tokengenerator.GenerateSignedToken(*identity0, kid0, authsupport.WithEmailClaim(email0), originalSubClaim)
		require.NoError(s.T(), err)

		claims, err := tokenParser.FromString(jwt0)
		require.NoError(s.T(), err)
		require.Equal(s.T(), identity0.Username, claims.PreferredUsername)
		require.Equal(s.T(), email0, claims.Email)
		require.Equal(s.T(), "OriginalSubValue:1234-ABCD", claims.OriginalSub)
	})

	s.Run("parse valid token with aud claim", func() {
		username0 := uuid.Must(uuid.NewV4()).String()
		identity0 := &authsupport.Identity{
			ID:       uuid.Must(uuid.NewV4()),
			Username: username0,
		}
		email0 := identity0.Username + "@email.tld"

		tests := map[string]struct {
			aud []string
		}{
			"single string": {
				aud: []string{"aud-claim-1"},
			},
			"multiple strings": {
				aud: []string{"aud-claim-1", "aud-claim-2"},
			},
		}

		for k, tc := range tests {
			s.T().Run(k, func(t *testing.T) {
				// generate non-serialized token
				jwt0 := tokengenerator.GenerateToken(*identity0, kid0, authsupport.WithEmailClaim(email0), authsupport.WithAudClaim(tc.aud))

				// serialize
				jwt0string, err := tokengenerator.SignToken(jwt0, kid0)
				require.NoError(s.T(), err)
				// validate token
				parsed, err := tokenParser.FromString(jwt0string)
				require.NoError(s.T(), err)
				require.Equal(s.T(), jwt.ClaimStrings(tc.aud), parsed.Audience)
			})
		}
	})
}
