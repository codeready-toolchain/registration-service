package cluster

import "time"

// TokenCluster represents a member cluster for the specific user token
type TokenCluster struct {
	Username    string
	ClusterName string
	ApiURL      string
	TokenHash   string
	Created     time.Time
}
