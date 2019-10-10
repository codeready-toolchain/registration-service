package context

const (
	// UsernameKey is the context key for preferred_username claim
	UsernameKey = "username"
	// EmailKey is the context key for email claim
	EmailKey = "email"
	// SubKey is the context key for subject claim
	SubKey = "subject"
	// JWTClaimsKey is the context key for the claims struct
	JWTClaimsKey = "jwtClaims"
)
