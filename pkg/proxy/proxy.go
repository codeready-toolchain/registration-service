package proxy

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/application"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/cluster"
)

type UserClusters struct {
	cacheByToken       map[string]*cluster.TokenCluster   // by token hash
	cacheByUserCluster map[string][]*cluster.TokenCluster // by username+apiURL hash. One user can have multiple tokens stored for one cluster
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
	tokenCluster, err := c.loadCluster(token)
	if err != nil {
		return "", err
	}
	return tokenCluster.ApiURL, nil
}

func (c *UserClusters) getUrlFromCache(token string) *string {
	th := tokenHash(token)
	c.mu.RLock()
	defer c.mu.RUnlock()
	tokenCluster, ok := c.cacheByToken[th]
	if ok {
		return &tokenCluster.ApiURL
	}
	return nil
}

func (c *UserClusters) loadCluster(token string) (*cluster.TokenCluster, error) {
	tokenCluster, err := c.app.ToolchainClusterService().Get(token)
	if err != nil {
		return nil, err
	}
	if tokenCluster == nil {
		// TODO return the first/random cluster URL or 401 with the message about expired token
		return nil, errors.New("cluster not found; the token might be expired")
	}

	// Cleanup existing expired token clusters for the same cluster & user if any
	ucHash := tokenClusterHash(tokenCluster.Username, tokenCluster.ApiURL)
	c.mu.Lock()
	defer c.mu.Unlock()
	existingTokenClusters, ok := c.cacheByUserCluster[ucHash]
	newTokenClusters := make([]*cluster.TokenCluster, 0)
	if ok {
		dayAgo := time.Now().Add(-time.Hour * 24)
		for _, cl := range existingTokenClusters {
			if cl.Created.Before(dayAgo) {
				// expired
				c.cacheByToken[cl.TokenHash] = nil
			} else {
				newTokenClusters = append(newTokenClusters, cl)
			}
		}
	}

	// Put the new cluster to the cache
	tokenCluster.TokenHash = tokenHash(token)
	c.cacheByToken[tokenCluster.TokenHash] = tokenCluster
	newTokenClusters = append(newTokenClusters, tokenCluster)
	c.cacheByUserCluster[ucHash] = newTokenClusters

	return tokenCluster, nil
}

func tokenHash(token string) string {
	return hash(token)
}

func tokenClusterHash(username, apiURL string) string {
	return hash(username + apiURL)
}

// hash calculates the md5 hash for the value
func hash(value string) string {
	md5hash := md5.New()
	// Ignore the error, as this implementation cannot return one
	_, _ = md5hash.Write([]byte(value))
	return hex.EncodeToString(md5hash.Sum(nil))
}
