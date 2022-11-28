package proxy

import (
	"fmt"
	"sync"

	"github.com/codeready-toolchain/registration-service/pkg/application"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/namespace"

	"github.com/gin-gonic/gin"
)

type UserClusters struct {
	sync.RWMutex
	app application.Application

	// cluster accesses by username
	clusterAccesses map[string]*namespace.ClusterAccess
}

func NewUserClusters(app application.Application) *UserClusters {
	return &UserClusters{
		app:             app,
		clusterAccesses: make(map[string]*namespace.ClusterAccess),
	}
}

// Get tries to retrieve the cluster access from the cache first. If found then it checks if the cached access is still valid.
// If not found or invalid then retrieves a new cluster access from the member cluster service and stores it in the cache.
func (c *UserClusters) Get(ctx *gin.Context, userID, username string) (*namespace.ClusterAccess, error) {
	ca, err := c.clientFromCache(ctx, username)
	if err != nil {
		return nil, err
	}
	if ca != nil {
		return ca, nil
	}

	// Not found in cache or invalid. Get a new cluster access.
	log.Info(ctx, fmt.Sprintf("Retrieving a new Cluster Access from the cluster for the user : %s", username))
	ca, err = c.app.MemberClusterService().GetClusterAccess(ctx, userID, username)
	if err != nil {
		return nil, err
	}
	// Store in the cache
	c.add(username, ca)

	return ca, nil
}

// namespaceFromCache tries to retrieve a cluster access from the cache and validates it.
// Returns nil, nil if no cluster access found or if it's invalid.
func (c *UserClusters) clientFromCache(ctx *gin.Context, username string) (*namespace.ClusterAccess, error) {
	c.RLock()
	defer c.RUnlock()
	ca, ok := c.clusterAccesses[username]
	if ok {
		valid, err := ca.Validate()
		if err != nil {
			return nil, err
		}
		if valid {
			log.Info(ctx, fmt.Sprintf("A valid Cluster Access found in cache for user '%s'", username))
			return ca, nil
		}
		log.Info(ctx, fmt.Sprintf("Cluster Access found in cache for user '%s' but it's not valid anymore", username))
		return nil, nil
	}
	log.Info(ctx, fmt.Sprintf("Cluster Access NOT found in cache for user : %s", username))
	return nil, nil
}

func (c *UserClusters) add(username string, ca *namespace.ClusterAccess) {
	c.Lock()
	defer c.Unlock()
	c.clusterAccesses[username] = ca
}
