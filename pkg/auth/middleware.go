package auth

import (
	"errors"
	"fmt"
	"log"
	"time"

	ginjwt "github.com/appleboy/gin-jwt/v2"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
)

// AuthMiddleware is the default auth middleware instance
var defaultAuthMiddleware *ginjwt.GinJWTMiddleware

// DefaultAuthMiddlewareWithConfig creates and stores a new auth middleware. Note that
// repeated calls to this will have no effect.
func DefaultAuthMiddlewareWithConfig(logger *log.Logger, config *configuration.Registry) (*ginjwt.GinJWTMiddleware, error) {
	var err error
	if defaultAuthMiddleware == nil {
		defaultAuthMiddleware, err = NewAuthMiddleware(logger, config)
		return defaultAuthMiddleware, err
	}
	return defaultAuthMiddleware, nil
}

// DefaultAuthMiddleware returns the existing auth middleware.
func DefaultAuthMiddleware() (*ginjwt.GinJWTMiddleware, error) {
	if defaultAuthMiddleware == nil {
		return nil, errors.New("no default auth middleware created, call `DefaultKeyManagerWithConfig()` first")
	}
	return defaultAuthMiddleware, nil
}

type User struct {
	UserName  string
	FirstName string
	LastName  string
}

type login struct {
	Username string `form:"username" json:"username" binding:"required"`
	Password string `form:"password" json:"password" binding:"required"`
}

// NewAuthMiddleware creates a new auth middleware.
func NewAuthMiddleware(logger *log.Logger, config *configuration.Registry) (*ginjwt.GinJWTMiddleware, error) {
	var identityKey = "id"
	return ginjwt.New(&ginjwt.GinJWTMiddleware{
		Realm:       "test zone",
		Key:         []byte("secret key"),
		Timeout:     time.Hour,
		MaxRefresh:  time.Hour,
		IdentityKey: identityKey,
		PayloadFunc: func(data interface{}) ginjwt.MapClaims {
			fmt.Println("############################## PayloadFunc")
			// PayloadFunc: maps the claims in the JWT
			if v, ok := data.(*User); ok {
				return ginjwt.MapClaims{
					identityKey: v.UserName,
				}
			}
			return ginjwt.MapClaims{}
		},
		IdentityHandler: func(c *gin.Context) interface{} {
			fmt.Println("############################## IdentityHandler")
			// IdentityHandler: extracts identity from claims.
			claims := ginjwt.ExtractClaims(c)
			return &User{
				UserName: claims[identityKey].(string),
			}
		},
		Authenticator: func(c *gin.Context) (interface{}, error) {
			fmt.Println("############################## Authenticator")
			var loginVals login
			if err := c.ShouldBind(&loginVals); err != nil {
				return "", jwt.ErrMissingLoginValues
			}
			userID := loginVals.Username
			password := loginVals.Password

			if (userID == "admin" && password == "admin") || (userID == "test" && password == "test") {
				return &User{
					UserName:  userID,
					LastName:  "Bo-Yi",
					FirstName: "Wu",
				}, nil
			}

			return nil, jwt.ErrFailedAuthentication
		},
		Authorizator: func(data interface{}, c *gin.Context) bool {
			fmt.Println("############################## Authorizator")
			// Authorizator: receives identity and handles authorization logic.
			if v, ok := data.(*User); ok && v.UserName == "admin" {
				return true
			}

			return false
		},
		Unauthorized: func(c *gin.Context, code int, message string) {
			fmt.Println("############################## Unauthorized")
			// Unauthorized: handles unauthorized logic.
			c.JSON(code, gin.H{
				"code":    code,
				"message": message,
			})
		},
		// TokenLookup is a string in the form of "<source>:<name>" that is used
		// to extract token from the request.
		// Optional. Default value "header:Authorization".
		// Possible values:
		// - "header:<name>"
		// - "query:<name>"
		// - "cookie:<name>"
		// - "param:<name>"
		TokenLookup: "header: Authorization, query: token, cookie: jwt",
		// TokenLookup: "query:token",
		// TokenLookup: "cookie:token",
		// TokenHeadName is a string in the header. Default value is "Bearer"
		TokenHeadName: "Bearer",
		// TimeFunc provides the current time. You can override it to use another time value. This is useful for testing or if your server uses a different time zone than your tokens.
		TimeFunc: time.Now,
	})
}
