package username

// Response represents the result of a search request done by using the MUR name or an email address.
// Based on the query string the response might contain multiple usernames.
type Response struct {
	// Matched is a list of the username matches found using the given search query.
	// There might be more than one match, for example when using email id for the search request.
	Matched []match `json:"matched,omitempty"`
}

type match struct {
	Username string `json:"username"`
}
