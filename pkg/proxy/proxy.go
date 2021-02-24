package proxy

import "sync"

type UserCluster struct {
	Username    string
	ClusterName string
	ApiURL      string
	TokenHash   string
}

type UserClusters struct {
	cacheByToken       map[string]*UserCluster // by token hash
	cacheByUserCluster map[string]*UserCluster // by userCluster hash
	mu                 sync.RWMutex
}

func (c *UserClusters) Url(token string) (string, error) {
	apiUrl := c.url(token)
	if apiUrl != nil {
		return *apiUrl, nil
	}
	userCluster, err := c.loadCluster(token)
	if err != nil {
		return "", err
	}
	return userCluster.ApiURL, nil
}

func (c *UserClusters) url(token string) *string {
	th := tokenHash(token)
	c.mu.RLock()
	defer c.mu.RUnlock()
	userCluster, ok := c.cacheByToken[th]
	if ok {
		return &userCluster.ApiURL
	}
	return nil
}

func (c *UserClusters) loadCluster(token string) (*UserCluster, error) {
	//TODO load all member clusters

	//TODO iterate member clusters and check whoami, so we know what cluster API URL and Username
	username := ""
	apiUrl := ""
	clusterName := ""

	// Cleanup existing cached tokens user clusters if any
	ucHash := userClusterHash(username, apiUrl)
	th := tokenHash(token)
	c.mu.Lock()
	defer c.mu.Unlock()
	userCluster, ok := c.cacheByUserCluster[ucHash]
	if ok {
		c.cacheByToken[userCluster.TokenHash] = nil
		c.cacheByUserCluster[ucHash] = nil
	}

	// Create UserCluster and put to cache
	userCluster = &UserCluster{
		Username:    username,
		ClusterName: clusterName,
		ApiURL:      apiUrl,
		TokenHash:   th,
	}
	c.cacheByToken[userCluster.TokenHash] = userCluster
	c.cacheByUserCluster[ucHash] = userCluster

	return userCluster, nil
}

func tokenHash(token string) string {
	//TODO
	return token
}

func userClusterHash(username, apiURL string) string {
	//TODO
	return username + apiURL
}
