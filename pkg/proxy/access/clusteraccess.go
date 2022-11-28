package access

import (
	"net/url"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterAccess holds an information needed to access the namespace in a member cluster for the specific user
type ClusterAccess struct { // nolint:revive
	// APIURL is the Cluster API Endpoint for the namespace
	apiURL url.URL
	// SAToken is a token of the Service Account which represents the user in the namespace
	saToken string
	// client is the kube client for the cluster.
	client client.Client
	// username is the id of the user to use for impersonation
	username string
}

func NewClusterAccess(apiURL url.URL, client client.Client, saToken, username string) *ClusterAccess {
	return &ClusterAccess{
		apiURL:   apiURL,
		client:   client,
		saToken:  saToken,
		username: username,
	}
}

func (a *ClusterAccess) APIURL() url.URL {
	return a.apiURL
}

func (a *ClusterAccess) SAToken() string {
	return a.saToken
}

func (a *ClusterAccess) Username() string {
	return a.username
}
