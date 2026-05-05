package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/codeready-toolchain/registration-service/pkg/auth"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/log"

	"github.com/labstack/echo/v4"
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

func (m *JWTMiddleware) extractToken(c echo.Context) (string, error) {
	headerToken := c.Request().Header.Get("Authorization")
	if headerToken != "" {
		if strings.HasPrefix(headerToken, "Bearer ") {
			s := strings.Fields(headerToken)
			if len(s) == 2 {
				return s[1], nil
			}
			return "", errors.New("found bearer token header, but no token:" + headerToken)
		}
		return "", errors.New("found unknown authorization header:" + headerToken)
	}
	return "", errors.New("no token found")
}

func (m *JWTMiddleware) respondWithError(c echo.Context, code int, message interface{}) error {
	return c.JSON(code, echo.Map{"error": message})
}

// HandlerFunc returns the Echo MiddlewareFunc.
func (m *JWTMiddleware) HandlerFunc() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			tokenStr, err := m.extractToken(c)
			if err != nil {
				return m.respondWithError(c, http.StatusUnauthorized, err.Error())
			}
			token, err := m.tokenParser.FromString(tokenStr)
			if err != nil {
				return m.respondWithError(c, http.StatusUnauthorized, err.Error())
			}

			if token.UserID == "" || token.AccountID == "" {
				rawClaims := ""

				parts := strings.Split(tokenStr, ".")

				if len(parts) >= 2 {
					rawClaims = parts[1]
				}

				log.Infof(c, "Missing essential claims from token - [user_id:%s][account_id:%s] for user [%s], sub [%s].  Raw claims segment: [%s]",
					token.UserID, token.AccountID, token.PreferredUsername, token.Subject, rawClaims)
			}

			c.Set(context.UserIDKey, token.UserID)
			c.Set(context.AccountIDKey, token.AccountID)
			c.Set(context.AccountNumberKey, token.AccountNumber)
			c.Set(context.UsernameKey, token.PreferredUsername)
			c.Set(context.EmailKey, token.Email)
			c.Set(context.SubKey, token.Subject)
			c.Set(context.OriginalSubKey, token.OriginalSub)
			c.Set(context.GivenNameKey, token.GivenName)
			c.Set(context.FamilyNameKey, token.FamilyName)
			c.Set(context.CompanyKey, token.Company)
			c.Set(context.JWTClaimsKey, token)
			return next(c)
		}
	}
}
