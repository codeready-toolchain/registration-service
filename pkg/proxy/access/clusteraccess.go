package access

import (
	"net/url"

	"github.com/codeready-toolchain/toolchain-common/pkg/usersignup"
)

// ClusterAccess holds information needed to access user namespaces in a member cluster for the specific user via impersonation
type ClusterAccess struct { // nolint:revive
	// APIURL is the Cluster API Endpoint for the namespace
	apiURL url.URL
	// impersonatorToken is a token of the impersonator's Service Account, usually the impersonator SA belongs to a member toolchaincluster
	impersonatorToken string
	// username is the id of the user to use for impersonation
	username string
}

func NewClusterAccess(apiURL url.URL, impersonatorToken, username string) *ClusterAccess {
	return &ClusterAccess{
		apiURL:            apiURL,
		impersonatorToken: impersonatorToken,
		username:          usersignup.TransformUsername(username),
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
