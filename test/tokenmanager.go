package test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
)

const (
	bitSize = 2048
)

// WebKeySet represents a JWK Set object.
type WebKeySet struct {
	Keys []jwk.Key `json:"keys"`
}

// PublicKey represents an RSA public key with a Key ID
type PublicKey struct {
	KeyID string
	Key   *rsa.PublicKey
}

// ExtraClaim a function to set claims in the token to generate
type ExtraClaim func(token *jwt.Token)

// WithEmailClaim sets the `email` claim in the token to generate
func WithEmailClaim(email string) ExtraClaim {
	return func(token *jwt.Token) {
		token.Claims.(jwt.MapClaims)["email"] = email
	}
}

// Identity is a user identity
type Identity struct {
	ID       uuid.UUID
	Username string
}

// NewIdentity returns a new, random identity
func NewIdentity() *Identity {
	return &Identity{
		ID:       uuid.NewV4(),
		Username: "testuser-" + uuid.NewV4().String(),
	}
}

// TokenManager represents the test token and key manager.
type TokenManager struct {
	keyMap map[string]*rsa.PrivateKey
}

// NewTokenManager creates a new TokenManager.
func NewTokenManager() *TokenManager {
	tg := &TokenManager{}
	tg.keyMap = make(map[string]*rsa.PrivateKey)
	return tg
}

// AddPrivateKey creates and stores a new key with the given kid.
func (tg *TokenManager) AddPrivateKey(kid string) (*rsa.PrivateKey, error) {
	reader := rand.Reader
	key, err := rsa.GenerateKey(reader, bitSize)
	if err != nil {
		return nil, err
	}
	tg.keyMap[kid] = key
	return key, nil
}

// RemovePrivateKey removes a key from the list of known keys.
func (tg *TokenManager) RemovePrivateKey(kid string) {
	delete(tg.keyMap, kid)
}

// Key retrieves the key associated with the given kid.
func (tg *TokenManager) Key(kid string) (*rsa.PrivateKey, error) {
	key, ok := tg.keyMap[kid]
	if !ok {
		return nil, errors.New("given kid does not exist")
	}
	return key, nil
}

// GenerateToken generates a default token.
func (tg *TokenManager) GenerateToken(identity Identity, kid string, extraClaims ...ExtraClaim) *jwt.Token {
	token := jwt.New(jwt.SigningMethodRS256)
	token.Claims.(jwt.MapClaims)["uuid"] = identity.ID
	token.Claims.(jwt.MapClaims)["preferred_username"] = identity.Username
	token.Claims.(jwt.MapClaims)["sub"] = identity.ID
	token.Claims.(jwt.MapClaims)["jti"] = uuid.NewV4().String()
	token.Claims.(jwt.MapClaims)["session_state"] = uuid.NewV4().String()
	token.Claims.(jwt.MapClaims)["iat"] = time.Now().Unix()
	token.Claims.(jwt.MapClaims)["exp"] = time.Now().Unix() + 60*60*24*30
	token.Claims.(jwt.MapClaims)["nbf"] = 0
	token.Claims.(jwt.MapClaims)["iss"] = "codeready-toolchain"
	token.Claims.(jwt.MapClaims)["typ"] = "Bearer"
	token.Claims.(jwt.MapClaims)["approved"] = true
	token.Claims.(jwt.MapClaims)["name"] = "Test User"
	token.Claims.(jwt.MapClaims)["company"] = "Company Inc."
	token.Claims.(jwt.MapClaims)["given_name"] = "Test"
	token.Claims.(jwt.MapClaims)["family_name"] = "User"
	token.Claims.(jwt.MapClaims)["email_verified"] = true
	for _, extra := range extraClaims {
		extra(token)
	}
	token.Header["kid"] = kid
	return token
}

// SignToken signs a given token using the given private key.
func (tg *TokenManager) SignToken(token *jwt.Token, kid string) (string, error) {
	key, err := tg.Key(kid)
	if err != nil {
		return "", err
	}
	tokenStr, err := token.SignedString(key)
	if err != nil {
		panic(errors.WithStack(err))
	}
	return tokenStr, nil
}

// GenerateSignedToken generates a JWT user token and signs it using the given private key.
func (tg *TokenManager) GenerateSignedToken(identity Identity, kid string, extraClaims ...ExtraClaim) (string, error) {
	token := tg.GenerateToken(identity, kid, extraClaims...)
	return tg.SignToken(token, kid)
}

// NewKeyServer creates and starts an http key server
func (tg *TokenManager) NewKeyServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		keySet := &WebKeySet{}
		for kid, key := range tg.keyMap {
			newKey, err := jwk.New(&key.PublicKey)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			err = newKey.Set(jwk.KeyIDKey, kid)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			keySet.Keys = append(keySet.Keys, newKey)
		}
		jsonKeyData, err := json.Marshal(keySet)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintln(w, string(jsonKeyData))
	}))
}

// NewJWKServer creates and starts a jwk server
func NewJWKServer(privateKey *rsa.PrivateKey, kid0 string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		publicKey := &privateKey.PublicKey

		keySet := &WebKeySet{}
		newKey, err := jwk.New(publicKey)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = newKey.Set(jwk.KeyIDKey, kid0)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		keySet.Keys = append(keySet.Keys, newKey)

		jsonKeyData, err := json.Marshal(keySet)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintln(w, string(jsonKeyData))
	}))
}
