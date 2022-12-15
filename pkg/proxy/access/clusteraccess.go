package access

import (
	"net/url"
)

// ClusterAccess holds information needed to access user namespaces in a member cluster for the specific user via impersonation
type ClusterAccess struct { // nolint:revive
	// APIURL is the Cluster API Endpoint for the namespace
	apiURL url.URL
	// impersonatorToken is a token of the impersonator's Service Account, usually the impersonator SA belongs to a member toolchaincluster
	impersonatorToken string
	// compliantUsername is the transformed username of the user (same as UserSignup.Status.CompliantUsername) to use for impersonation
	compliantUsername string
}

func NewClusterAccess(apiURL url.URL, impersonatorToken, compliantUsername string) *ClusterAccess {
	return &ClusterAccess{
		apiURL:            apiURL,
		impersonatorToken: impersonatorToken,
		compliantUsername: compliantUsername,
	}
}

func (a *ClusterAccess) APIURL() url.URL {
	return a.apiURL
}

func (a *ClusterAccess) ImpersonatorToken() string {
	return a.impersonatorToken
}

func (a *ClusterAccess) CompliantUsername() string {
	return a.compliantUsername
}
