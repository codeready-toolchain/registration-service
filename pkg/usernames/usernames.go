package usernames

// Response represents the result of a search request done by using the MUR name or an email address.
// The result will specify if a user exits in the system or not , or if there are multiple matches for the given search query
type Response struct {
	// For is the search string that was specified in the search request url. Example: /usernames/<search string>
	For string `json:"for"`
	// Valid can be true or false.
	// 	true indicates that a match was found
	// 	false indicates that no match was found
	Valid string `json:"valid"`
	// Matched is a list of the username matches found using the given search query.
	// There might be more than one match, for example when using email id for the search request.
	Matched []match `json:"matched,omitempty"`
}

type match struct {
	Username string `json:"username"`
}
