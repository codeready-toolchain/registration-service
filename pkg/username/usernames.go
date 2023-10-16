package username

// Response represents the result of a search request done by using the MUR name or an email address.
// Based on the query string the response might contain multiple usernames or no username at all.
type Response []struct {
	Username string `json:"username,omitempty"`
}
