package middleware

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/codeready-toolchain/registration-service/pkg/auth"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
)

const (
	// UsernameKey is the context key for preferred_username claim
	UsernameKey = "username"
	// EmailKey is the context key for email claim
	EmailKey = "email"
	// JWTClaimsKey is the context key for the claims struct
	JWTClaimsKey = "jwtClaims"
)

// JWTMiddleware is the JWT token validation middleware
type JWTMiddleware struct {
	config      *configuration.Registry
	logger      *log.Logger
	keyManager  *auth.KeyManager
	tokenParser *auth.TokenParser
}

// NewAuthMiddleware returns a new middleware for JWT authentication
func NewAuthMiddleware(logger *log.Logger, config *configuration.Registry) (*JWTMiddleware, error) {
	if logger == nil || config == nil {
		return nil, errors.New("missing parameters for NewAuthMiddleware")
	}
	// wire up the key and token management
	keyManagerInstance, err := auth.DefaultKeyManager()
	if err != nil {
		return nil, err
	}
	tokenParserInstance, err := auth.DefaultTokenParser()
	if err != nil {
		return nil, err
	}
	return &JWTMiddleware{
		logger:      logger,
		config:      config,
		keyManager:  keyManagerInstance,
		tokenParser: tokenParserInstance,
	}, nil
}

func (m *JWTMiddleware) extractToken(c *gin.Context) (string, error) {
	// token lookup order: header: Authorization, query: token
	// try header field "Authorization" (will be "" when n/a)
	headerToken := c.GetHeader("Authorization")
	if headerToken != "" {
		if strings.HasPrefix(headerToken, "Bearer ") {
			// it is a bearer token, split it up and return it
			s := strings.Fields(headerToken)
			if len(s) == 2 {
				return s[1], nil
			}
			// we're failing fast here, if there is an Authorization header, it is used or it fails
			return "", errors.New("found bearer token header, but no token:" + headerToken)
		}
		// see above, failing fast
		return "", errors.New("found unknown authorization header:" + headerToken)
	}
	// next, try GET param "token" (will return "" if n/a)
	paramToken := c.Query("token")
	if paramToken != "" {
		return paramToken, nil
	}
	return "", errors.New("no token found")
}

func (m *JWTMiddleware) respondWithError(c *gin.Context, code int, message interface{}) {
	c.AbortWithStatusJSON(code, gin.H{"error": message})
}

// HandlerFunc returns the HanderFunc.
func (m *JWTMiddleware) HandlerFunc() gin.HandlerFunc {
	return func(c *gin.Context) {
		// check if we have a token
		tokenStr, err := m.extractToken(c)
		if err != nil {
			m.respondWithError(c, http.StatusForbidden, err.Error())
			c.Abort()
			return
		}
		// next, check the token
		token, err := m.tokenParser.FromString(tokenStr)
		if err != nil {
			m.respondWithError(c, http.StatusForbidden, err.Error())
			c.Abort()
			return
		}
		// validate time claims
		if token.Valid() != nil {
			m.respondWithError(c, http.StatusForbidden, "token has invalid time claims")
			c.Abort()
			return
		}
		// check if we have the needed claims for username and email
		if token.Username == "" {
			m.respondWithError(c, http.StatusForbidden, "token does not have preferred_username set")
			c.Abort()
			return
		}
		if token.Email == "" {
			m.respondWithError(c, http.StatusForbidden, "token does not have email set")
			c.Abort()
			return
		}
		// all checks done, add username and email to the context
		c.Set(UsernameKey, token.Username)
		c.Set(EmailKey, token.Email)
		// for convenience, add the claims to the context
		c.Set(JWTClaimsKey, token)
		c.Next()
	}
}
