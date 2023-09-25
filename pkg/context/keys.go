package context

const (
	// UserIDKey is the context key for the user_id claim
	UserIDKey = "user_id"
	// AccountIDKey is the context key for the account_id claim
	AccountIDKey = "account_id"
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
)
