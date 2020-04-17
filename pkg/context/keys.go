package context

const (
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
	// JWTClaimsKey is the context key for the claims struct
	JWTClaimsKey = "jwtClaims"
)
