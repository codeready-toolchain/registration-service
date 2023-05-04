package auth

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/golang-jwt/jwt"
)

/****************************************************

  This section is a temporary fix until formal leeway support is available in the next jwt-go release

 *****************************************************/

const leeway = 5000

// TokenClaims represents access token claims
type TokenClaims struct {
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
	GivenName         string `json:"given_name"`
	FamilyName        string `json:"family_name"`
	Email             string `json:"email"`
	EmailVerified     bool   `json:"email_verified"`
	Company           string `json:"company"`
	OriginalSub       string `json:"original_sub"`
	UserID            string `json:"user_id"`
	AccountID         string `json:"account_id"`
	jwt.StandardClaims
}

// Valid checks whether the token claims are valid
func (c *TokenClaims) Valid() error {
	c.StandardClaims.IssuedAt -= leeway
	now := time.Now().Unix()
	err := c.StandardClaims.Valid()
	if err != nil {
		log.Error(nil, err, "Token validation failed")
		log.Infof(nil, "Current time: %s", strconv.FormatInt(now, 10))
		log.Infof(nil, "Token IssuedAt time: %s", strconv.FormatInt(c.StandardClaims.IssuedAt, 10))
	}
	c.StandardClaims.IssuedAt += leeway
	return err
}

// TokenParser represents a parser for JWT tokens.
type TokenParser struct {
	keyManager *KeyManager
}

// NewTokenParser creates a new TokenParser.
func NewTokenParser(keyManager *KeyManager) (*TokenParser, error) {
	if keyManager == nil {
		return nil, errors.New("no keyManager given when creating TokenParser")
	}
	return &TokenParser{
		keyManager: keyManager,
	}, nil
}

// FromString parses a JWT, validates the signature and returns the claims struct.
func (tp *TokenParser) FromString(jwtEncoded string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(jwtEncoded, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		// validate the alg is what we expect
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

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
		if claims.PreferredUsername == "" {
			return nil, errors.New("token does not comply to expected claims: username missing")
		}
		if claims.Email == "" {
			return nil, errors.New("token does not comply to expected claims: email missing")
		}
		if claims.Subject == "" {
			return nil, errors.New("token does not comply to expected claims: subject missing")
		}
		return claims, nil
	}
	return nil, errors.New("token does not comply to expected claims")
}
