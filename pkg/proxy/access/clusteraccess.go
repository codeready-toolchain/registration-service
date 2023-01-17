package access

import (
	"net/url"
)

// ClusterAccess holds information needed to access user namespaces in a member cluster for the specific user via impersonation
type ClusterAccess struct { // nolint:revive
	// APIURL is the Cluster API Endpoint for the namespace
	apiURL url.URL
	// impersonatorToken is a token of the Service Account with impersonation role, typically the member toolchaincluster SA
	impersonatorToken string
	// username is the id of the user to use for impersonation
	username string
}

func NewClusterAccess(apiURL url.URL, impersonatorToken, username string) *ClusterAccess {
	return &ClusterAccess{
		apiURL:            apiURL,
		impersonatorToken: impersonatorToken,
		username:          username,
	}
}

func (a *ClusterAccess) APIURL() url.URL {
	return a.apiURL
}

func (a *ClusterAccess) ImpersonatorToken() string {
	return a.impersonatorToken
}

func (a *ClusterAccess) Username() string {
	return a.username
}
