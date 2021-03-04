package cluster

// UserCluster represents a member cluster for the specific user/token
type UserCluster struct {
	Username    string
	ClusterName string
	ApiURL      string
	TokenHash   string
}
