package signup

import (
	"errors"
	"log"

	jwt "github.com/dgrijalva/jwt-go"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
)

// TokenClaims represents access token claims
type TokenClaims struct {
	Name          string         `json:"name"`
	Username      string         `json:"preferred_username"`
	GivenName     string         `json:"given_name"`
	FamilyName    string         `json:"family_name"`
	Email         string         `json:"email"`
	EmailVerified bool           `json:"email_verified"`
	Company       string         `json:"company"`
	SessionState  string         `json:"session_state"`
	Approved      bool           `json:"approved"`
	Permissions   *[]Permissions `json:"permissions"`
	jwt.StandardClaims
}

// Permissions represents a "permissions" claim in the AuthorizationPayload
type Permissions struct {
	ResourceSetName *string  `json:"resource_set_name"`
	ResourceSetID   *string  `json:"resource_set_id"`
	Scopes          []string `json:"scopes"`
	Expiry          int64    `json:"exp"`
}

// TokenParser represents a token.
type TokenParser struct {
	config     *configuration.Registry
	logger     *log.Logger
	keyManager *KeyManager
}

// NewTokenParser creates a new TokenParser.
func NewTokenParser(logger *log.Logger, config *configuration.Registry, keyManager *KeyManager) (*TokenParser, error) {
	if logger == nil {
		return nil, errors.New("no logger given when creating TokenParser")
	}
	if config == nil {
		return nil, errors.New("no config given when creating TokenParser")
	}
	if keyManager == nil {
		return nil, errors.New("no keyManager given when creating TokenParser")
	}
	return &TokenParser{
		logger:     logger,
		config:     config,
		keyManager: keyManager,
	}, nil
}

// FromString parses a JWT, validates the signaure and returns the claims struct.
func (tp *TokenParser) FromString(jwtEncoded string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(jwtEncoded, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		kid := token.Header["kid"]
		if kid == nil {
			return nil, errors.New("no key id given in the token")
		}
		kidStr, ok := kid.(string)
		if !ok {
			return nil, errors.New("given key id has unknown type")
		}
		// get the public key for kid from keyManager
		publicKey, err := tp.keyManager.Key(kidStr)
		if err != nil {
			return nil, err
		}
		return publicKey, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*TokenClaims); ok && token.Valid {
		// we need username and email, so check if those are contained in the claims
		if claims.Username == "" {
			return nil, errors.New("token does not comply to expected claims: username missing")
		}
		if claims.Email == "" {
			return nil, errors.New("token does not comply to expected claims: email missing")
		}
		return claims, nil
	}
	return nil, errors.New("token does not comply to expected claims")
}
