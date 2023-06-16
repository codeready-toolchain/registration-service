package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/codeready-toolchain/registration-service/pkg/auth"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/log"

	"github.com/gin-gonic/gin"
)

// JWTMiddleware is the JWT token validation middleware
type JWTMiddleware struct {
	tokenParser *auth.TokenParser
}

// NewAuthMiddleware returns a new middleware for JWT authentication
func NewAuthMiddleware() (*JWTMiddleware, error) {
	tokenParserInstance, err := auth.DefaultTokenParser()
	if err != nil {
		return nil, err
	}
	return &JWTMiddleware{
		tokenParser: tokenParserInstance,
	}, nil
}

func (m *JWTMiddleware) extractToken(c *gin.Context) (string, error) {
	// token lookup: header: Authorization
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
			m.respondWithError(c, http.StatusUnauthorized, err.Error())
			return
		}
		// next, check the token
		token, err := m.tokenParser.FromString(tokenStr)
		if err != nil {
			m.respondWithError(c, http.StatusUnauthorized, err.Error())
			return
		}

		if token.UserID == "" || token.AccountID == "" {
			log.Infof(c, "Missing essential claims from token - [user_id:%s][account_id:%s] for user [%s], sub [%s]",
				token.UserID, token.AccountID, token.PreferredUsername, token.Subject)
		}

		// all checks done, add username, subject and email to the context.
		// the tokenparser has already checked these claims are in the token at this point.
		c.Set(context.UserIDKey, token.UserID)
		c.Set(context.AccountIDKey, token.AccountID)
		c.Set(context.UsernameKey, token.PreferredUsername)
		c.Set(context.EmailKey, token.Email)
		c.Set(context.SubKey, token.Subject)
		c.Set(context.OriginalSubKey, token.OriginalSub)
		c.Set(context.GivenNameKey, token.GivenName)
		c.Set(context.FamilyNameKey, token.FamilyName)
		c.Set(context.CompanyKey, token.Company)
		// for convenience, add the claims to the context.
		c.Set(context.JWTClaimsKey, token)
		c.Next()
	}
}
