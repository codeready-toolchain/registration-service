package access

import (
	"net/url"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterAccess holds an information needed to access the namespace in a member cluster for the specific user
type ClusterAccess struct { // nolint:revive
	// APIURL is the Cluster API Endpoint for the namespace
	apiURL url.URL
	// impersonatorToken is a token of the impersonator's Service Account, usually the impersonator SA belongs to a member toolchaincluster
	impersonatorToken string
	// client is the kube client for the cluster.
	client client.Client
	// username is the id of the user to use for impersonation
	username string
}

func NewClusterAccess(apiURL url.URL, client client.Client, impersonatorToken, username string) *ClusterAccess {
	return &ClusterAccess{
		apiURL:            apiURL,
		client:            client,
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
