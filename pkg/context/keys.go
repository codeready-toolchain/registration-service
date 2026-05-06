package context

import (
	gocontext "context"

	"github.com/labstack/echo/v4"
)

// RequestContext extracts the standard context.Context from an Echo request
// for use with Kubernetes client calls. Falls back to context.TODO() when the
// Echo context or its underlying request is nil (e.g. in tests).
func RequestContext(ctx echo.Context) gocontext.Context {
	if ctx != nil && ctx.Request() != nil {
		return ctx.Request().Context()
	}
	return gocontext.TODO()
}

// GetString returns the string value associated with the given key in the echo context.
// Returns empty string if the key doesn't exist or the value is not a string.
func GetString(ctx echo.Context, key string) string {
	if ctx == nil {
		return ""
	}
	if val, ok := ctx.Get(key).(string); ok {
		return val
	}
	return ""
}

const (
	// UserIDKey is the context key for the user_id claim
	UserIDKey = "user_id"
	// AccountIDKey is the context key for the account_id claim
	AccountIDKey = "account_id"
	// AccountNumberKey is the context key for the account_number claim
	AccountNumberKey = "account_number"
	// UsernameKey is the context key for the preferred_username claim
	UsernameKey = "username"
	// EmailKey is the context key for the email claim
	EmailKey = "email"
	// GivenNameKey is the context key for the given name claim
	GivenNameKey = "givenName"
	// FamilyNameKey is the context key for the family name claim
	FamilyNameKey = "familyName"
	// CompanyKey is the context key for the company claim
	CompanyKey = "company"
	// SubKey is the context key for the subject claim
	SubKey = "subject"
	// OriginalSubKey is the context key for the original subject claim
	OriginalSubKey = "originalSub"
	// JWTClaimsKey is the context key for the claims struct
	JWTClaimsKey = "jwtClaims"
	// WorkspaceKey is the context key for the workspace name in echo.Context
	WorkspaceKey = "workspace"
	// RequestReceivedTime is the context key for the starting time of a request made
	RequestReceivedTime = "requestReceivedTime"
	// PublicViewerEnabled is a boolean value indicating whether PublicViewer support is enabled
	PublicViewerEnabled = "publicViewerEnabled"
	// ImpersonateUser is the context key for the impersonated user in proxied call
	ImpersonateUser = "impersonateUser"
	// SocialEvent is the context key for the activation code provided in UI
	SocialEvent = "socialEvent"
)
