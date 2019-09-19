package testutils

import (
	"bytes"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/dgrijalva/jwt-go"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/require"
	jose "gopkg.in/square/go-jose.v2"
	"github.com/stretchr/testify/suite"
)

type TestTokenManagerSuite struct {
	UnitTestSuite
}

 func TestRunTokenManagerSuite(t *testing.T) {
	suite.Run(t, &TestTokenManagerSuite{UnitTestSuite{}})
}

func (s *TestTokenManagerSuite) TestTokenManagerKeys() {

	s.Run("create keys", func() {
		tokenManager := NewTokenManager()
		kid0 := uuid.NewV4().String()
		key0, err := tokenManager.AddPrivateKey(kid0)
		require.NoError(s.T(), err)
		require.NotNil(s.T(), key0)
		kid1 := uuid.NewV4().String()
		key1, err := tokenManager.AddPrivateKey(kid1)
		require.NoError(s.T(), err)
		require.NotNil(s.T(), key1)
		// check key equality by comparing the modulus
		require.NotEqual(s.T(), key0.N, key1.N)
	})

	s.Run("get key", func() {
		tokenManager := NewTokenManager()
		kid0 := uuid.NewV4().String()
		key0, err := tokenManager.AddPrivateKey(kid0)
		require.NoError(s.T(), err)
		require.NotNil(s.T(), key0)
		kid1 := uuid.NewV4().String()
		key1, err := tokenManager.AddPrivateKey(kid1)
		require.NoError(s.T(), err)
		require.NotNil(s.T(), key1)
		key0Retrieved, err := tokenManager.Key(kid0)
		require.NoError(s.T(), err)
		require.NotNil(s.T(), key0Retrieved)
		// check key equality by comparing the modulus
		require.Equal(s.T(), key0.N, key0Retrieved.N)
		key1Retrieved, err := tokenManager.Key(kid1)
		require.NoError(s.T(), err)
		require.NotNil(s.T(), key1Retrieved)
		// check key equality by comparing the modulus
		require.Equal(s.T(), key1.N, key1Retrieved.N)
	})
}

func (s *TestTokenManagerSuite) TestTokenManagerTokens() {
	tokenManager := NewTokenManager()
	kid0 := uuid.NewV4().String()
	key0, err := tokenManager.AddPrivateKey(kid0)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), key0)

	s.Run("create token", func() {
		username := uuid.NewV4().String()
		identity0 := &Identity{
			ID:       uuid.NewV4(),
			Username: username,
		}
		// generate the token
		encodedToken, err := tokenManager.GenerateSignedToken(*identity0, kid0)
		require.NoError(s.T(), err)
		// unmarshall it again
		decodedToken, err := jwt.ParseWithClaims(encodedToken, &jwt.StandardClaims{}, func(token *jwt.Token) (interface{}, error) {
			return &(key0.PublicKey), nil
		})
		require.NoError(s.T(), err)
		require.True(s.T(), decodedToken.Valid)
		claims, ok := decodedToken.Claims.(*jwt.StandardClaims)
		require.True(s.T(), ok)
		require.Equal(s.T(), identity0.ID.String(), claims.Subject)
	})

	s.Run("create token with email extra claim", func() {
		username := uuid.NewV4().String()
		identity0 := &Identity{
			ID:       uuid.NewV4(),
			Username: username,
		}
		// generate the token
		encodedToken, err := tokenManager.GenerateSignedToken(*identity0, kid0, WithEmailClaim(identity0.Username + "@email.tld"))
		require.NoError(s.T(), err)
		// unmarshall it again
		decodedToken, err := jwt.ParseWithClaims(encodedToken, &jwt.StandardClaims{}, func(token *jwt.Token) (interface{}, error) {
			return &(key0.PublicKey), nil
		})
		require.NoError(s.T(), err)
		require.True(s.T(), decodedToken.Valid)
		claims, ok := decodedToken.Claims.(*jwt.StandardClaims)
		require.True(s.T(), ok)
		require.Equal(s.T(), identity0.ID.String(), claims.Subject)
	})
}

func (s *TestTokenManagerSuite) TestTokenManagerKeyService() {
	tokenManager := NewTokenManager()
	kid0 := uuid.NewV4().String()
	key0, err := tokenManager.AddPrivateKey(kid0)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), key0)
	kid1 := uuid.NewV4().String()
	key1, err := tokenManager.AddPrivateKey(kid1)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), key1)

	s.Run("key fetching", func() {
		ks := tokenManager.NewKeyServer()
		defer ks.Close()
		keysEndpointURL := ks.URL
		httpClient := http.DefaultClient
		req, err := http.NewRequest("GET", keysEndpointURL, nil)
		require.NoError(s.T(), err)
		res, err := httpClient.Do(req)
		require.NoError(s.T(), err)
		// read and parse response body
		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(res.Body)
		require.NoError(s.T(), err)
		bodyBytes := buf.Bytes()

		// if status code was not OK, bail out
		require.Equal(s.T(), http.StatusOK, res.StatusCode)

		// unmarshal the keys
		// note: we're intentionally using jose here, not jwx to have two
		// different jwt implementations interact and to not miss implementation
		// or standards issues in the jose library.
		webKeys := &jose.JSONWebKeySet{}
		err = json.Unmarshal(bodyBytes, &webKeys)
		require.NoError(s.T(), err)

		// check key integrity for key 0
		webKey0 := webKeys.Key(kid0)
		require.NotNil(s.T(), webKey0)
		require.Equal(s.T(), 1, len(webKey0))
		rsaKey0, ok := webKey0[0].Key.(*rsa.PublicKey)
		require.True(s.T(), ok)
		// check key equality by comparing the modulus
		require.Equal(s.T(), key0.N, rsaKey0.N)

		// check key integrity for key 1
		webKey1 := webKeys.Key(kid1)
		require.NotNil(s.T(), webKey1)
		require.Equal(s.T(), 1, len(webKey1))
		rsaKey1, ok := webKey1[0].Key.(*rsa.PublicKey)
		require.True(s.T(), ok)
		// check key equality by comparing the modulus
		require.Equal(s.T(), key1.N, rsaKey1.N)
	})
}
