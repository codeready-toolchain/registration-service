package namespace

import "net/url"

// Namespace represents a namespace in a member cluster for the specific sso user-workspace
type Namespace struct {
	Username    string
	ClusterName string
	APIURL      url.URL
	Namespace   string
	Workspace   string
	// Target cluster token
	TargetClusterToken string
}
