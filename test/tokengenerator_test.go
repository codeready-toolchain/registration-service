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
)

func TestTokenGeneratorKeys(t *testing.T) {

	t.Run("create keys", func(t *testing.T) {
		tokengenerator := NewTokenGenerator()
		kid0 := uuid.NewV4().String()
		key0, err := tokengenerator.CreateKey(kid0)
		require.NoError(t, err)
		kid1 := uuid.NewV4().String()
		key1, err := tokengenerator.CreateKey(kid1)
		require.NoError(t, err)
		// check key equality by comparing the modulus
		require.NotEqual(t, key0.N, key1.N)
	})

	t.Run("get key", func(t *testing.T) {
		tokengenerator := NewTokenGenerator()
		kid0 := uuid.NewV4().String()
		key0, err := tokengenerator.CreateKey(kid0)
		require.NoError(t, err)
		kid1 := uuid.NewV4().String()
		key1, err := tokengenerator.CreateKey(kid1)
		require.NoError(t, err)
		key0Retrieved, err := tokengenerator.Key(kid0)
		require.NoError(t, err)
		// check key equality by comparing the modulus
		require.Equal(t, key0.N, key0Retrieved.N)
		key1Retrieved, err := tokengenerator.Key(kid1)
		require.NoError(t, err)
		// check key equality by comparing the modulus
		require.Equal(t, key1.N, key1Retrieved.N)
	})
}

func TestTokenGeneratorTokens(t *testing.T) {
	tokengenerator := NewTokenGenerator()
	kid0 := uuid.NewV4().String()
	key0, err := tokengenerator.CreateKey(kid0)
	require.NoError(t, err)

	t.Run("create token", func(t *testing.T) {
		username := uuid.NewV4().String()
		identity0 := &Identity{
			ID:       uuid.NewV4(),
			Username: username,
		}
		// generate the token
		encodedToken, err := tokengenerator.GenerateSignedToken(*identity0, kid0)
		require.NoError(t, err)
		// unmarshall it again
		decodedToken, err := jwt.ParseWithClaims(encodedToken, &jwt.StandardClaims{}, func(token *jwt.Token) (interface{}, error) {
			return &(key0.PublicKey), nil
		})
		require.NoError(t, err)
		require.True(t, decodedToken.Valid)
		claims, ok := decodedToken.Claims.(*jwt.StandardClaims)
		require.True(t, ok)
		require.Equal(t, identity0.ID.String(), claims.Subject)
	})

	t.Run("create token with email extra claim", func(t *testing.T) {
		username := uuid.NewV4().String()
		identity0 := &Identity{
			ID:       uuid.NewV4(),
			Username: username,
		}
		// generate the token
		encodedToken, err := tokengenerator.GenerateSignedToken(*identity0, kid0, WithEmailClaim(identity0.Username + "@email.tld"))
		require.NoError(t, err)
		// unmarshall it again
		decodedToken, err := jwt.ParseWithClaims(encodedToken, &jwt.StandardClaims{}, func(token *jwt.Token) (interface{}, error) {
			return &(key0.PublicKey), nil
		})
		require.NoError(t, err)
		require.True(t, decodedToken.Valid)
		claims, ok := decodedToken.Claims.(*jwt.StandardClaims)
		require.True(t, ok)
		require.Equal(t, identity0.ID.String(), claims.Subject)
	})
}

func TestTokenGeneratorKeyService(t *testing.T) {
	tokengenerator := NewTokenGenerator()
	kid0 := uuid.NewV4().String()
	key0, err := tokengenerator.CreateKey(kid0)
	require.NoError(t, err)
	kid1 := uuid.NewV4().String()
	key1, err := tokengenerator.CreateKey(kid1)
	require.NoError(t, err)

	t.Run("key fetching", func(t *testing.T) {
		keysEndpointURL := tokengenerator.GetKeyService()
		httpClient := http.DefaultClient
		req, err := http.NewRequest("GET", keysEndpointURL, nil)
		require.NoError(t, err)
		res, err := httpClient.Do(req)
		require.NoError(t, err)
		// read and parse response body
		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(res.Body)
		require.NoError(t, err)
		bodyBytes := buf.Bytes()

		// if status code was not OK, bail out
		require.Equal(t, http.StatusOK, res.StatusCode)

		// unmarshal the keys
		// note: we're intentionally using jose here, not jwx to have two
		// different jwt implementations interact and to not miss implementation
		// or standards issues in the jose library.
		webKeys := &jose.JSONWebKeySet{}
		err = json.Unmarshal(bodyBytes, &webKeys)
		require.NoError(t, err)

		// check key integrity for key 0
		webKey0 := webKeys.Key(kid0)
		require.NotNil(t, webKey0)
		require.Equal(t, 1, len(webKey0))
		rsaKey0, ok := webKey0[0].Key.(*rsa.PublicKey)
		require.True(t, ok)
		// check key equality by comparing the modulus
		require.Equal(t, key0.N, rsaKey0.N)

		// check key integrity for key 1
		webKey1 := webKeys.Key(kid1)
		require.NotNil(t, webKey1)
		require.Equal(t, 1, len(webKey1))
		rsaKey1, ok := webKey1[0].Key.(*rsa.PublicKey)
		require.True(t, ok)
		// check key equality by comparing the modulus
		require.Equal(t, key1.N, rsaKey1.N)
	})
}
