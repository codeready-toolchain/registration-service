package namespace

import "net/url"

// Namespace represents a namespace in a member cluster for the specific sso user-workspace
type Namespace struct {
	APIURL url.URL
	// TargetClusterToken is a token of the Service Account which represents the user in the target member cluster namespace
	TargetClusterToken string
}
