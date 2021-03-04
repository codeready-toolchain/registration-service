package proxy

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"sync"

	"github.com/codeready-toolchain/registration-service/pkg/application"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/cluster"
)

type UserClusters struct {
	cacheByToken       map[string]*cluster.UserCluster // by token hash
	cacheByUserCluster map[string]*cluster.UserCluster // by userCluster hash
	mu                 sync.RWMutex
	app                application.Application
}

func NewUserClusters(app application.Application) *UserClusters {
	return &UserClusters{
		app: app,
	}
}

func (c *UserClusters) Url(token string) (string, error) {
	apiUrl := c.getUrlFromCache(token)
	if apiUrl != nil {
		return *apiUrl, nil
	}
	userCluster, err := c.loadCluster(token)
	if err != nil {
		return "", err
	}
	return userCluster.ApiURL, nil
}

func (c *UserClusters) getUrlFromCache(token string) *string {
	th := tokenHash(token)
	c.mu.RLock()
	defer c.mu.RUnlock()
	userCluster, ok := c.cacheByToken[th]
	if ok {
		return &userCluster.ApiURL
	}
	return nil
}

func (c *UserClusters) loadCluster(token string) (*cluster.UserCluster, error) {
	userCluster, err := c.app.ToolchainClusterService().Get(token)
	if err != nil {
		return nil, err
	}
	if userCluster == nil {
		// TODO return the first/random cluster URL or 401 with the message about expired token
		return nil, errors.New("cluster not found; the token might be expired")
	}

	// Cleanup existing cached tokens user clusters if any
	ucHash := userClusterHash(userCluster.Username, userCluster.ApiURL)
	th := tokenHash(token)
	c.mu.Lock()
	defer c.mu.Unlock()
	existingUserCluster, ok := c.cacheByUserCluster[ucHash]
	if ok {
		c.cacheByToken[existingUserCluster.TokenHash] = nil
		c.cacheByUserCluster[ucHash] = nil
	}

	// Put the new cluster to the cache
	userCluster.TokenHash = th
	c.cacheByToken[userCluster.TokenHash] = userCluster
	c.cacheByUserCluster[ucHash] = userCluster

	return userCluster, nil
}

func tokenHash(token string) string {
	return hash(token)
}

func userClusterHash(username, apiURL string) string {
	return hash(username + apiURL)
}

// hash calculates the md5 hash for the value
func hash(value string) string {
	md5hash := md5.New()
	// Ignore the error, as this implementation cannot return one
	_, _ = md5hash.Write([]byte(value))
	return hex.EncodeToString(md5hash.Sum(nil))
}
