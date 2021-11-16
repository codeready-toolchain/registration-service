package namespace

import "net/url"

// NamespaceAccess holds an information needed to access the namespace in a member cluster for the specific user
type NamespaceAccess struct {
	// APIURL is the Cluster API Endpoint for the namespace
	APIURL url.URL
	// SAToken is a token of the Service Account which represents the user in the namespace
	SAToken string
}
