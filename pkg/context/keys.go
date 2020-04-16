package context

const (
	// UsernameKey is the context key for the preferred_username claim
	UsernameKey = "username"
	// EmailKey is the context key for the email claim
	EmailKey = "email"
	// NameKey is the context key for the given name claim
	GivenName = "givenName"
	// FamilyNameKey is the context key for the family name claim
	FamilyNameKey = "familyName"
	// Company is the context key for the company claim
	Company = "company"
	// SubKey is the context key for the subject claim
	SubKey = "subject"
	// JWTClaimsKey is the context key for the claims struct
	JWTClaimsKey = "jwtClaims"
)
