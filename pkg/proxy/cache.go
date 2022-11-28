package proxy

import (
	"fmt"
	"sync"

	"github.com/codeready-toolchain/registration-service/pkg/application"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/access"

	"github.com/gin-gonic/gin"
)

type UserAccess struct {
	sync.RWMutex
	app application.Application

	// cluster accesses by username
	clusterAccesses map[string]*access.ClusterAccess
}

func NewUserAccess(app application.Application) *UserAccess {
	return &UserAccess{
		app:             app,
		clusterAccesses: make(map[string]*access.ClusterAccess),
	}
}

// Get tries to retrieve the cluster access from the cache first. If found then it checks if the cached access is still valid.
// If not found or invalid then retrieves a new cluster access from the member cluster service and stores it in the cache.
func (c *UserAccess) Get(ctx *gin.Context, userID, username string) (*access.ClusterAccess, error) {
	ca := c.clusterAccessFromCache(ctx, username)
	if ca != nil {
		return ca, nil
	}

	// Not found in cache or invalid. Get a new cluster access.
	log.Info(ctx, fmt.Sprintf("Retrieving a new Cluster Access from the cluster for the user : %s", username))
	ca, err := c.app.MemberClusterService().GetClusterAccess(ctx, userID, username)
	if err != nil {
		return nil, err
	}
	// Store in the cache
	c.add(username, ca)

	return ca, nil
}

// clusterAccessFromCache tries to retrieve a cluster access from the cache and validates it.
// Returns nil if no cluster access found.
func (c *UserAccess) clusterAccessFromCache(ctx *gin.Context, username string) *access.ClusterAccess {
	c.RLock()
	defer c.RUnlock()
	ca, ok := c.clusterAccesses[username]
	if !ok {
		log.Info(ctx, fmt.Sprintf("Cluster Access NOT found in cache for user : %s", username))
		return nil
	}
	log.Info(ctx, fmt.Sprintf("A valid Cluster Access found in cache for user '%s'", username))
	return ca
}

func (c *UserAccess) add(username string, ca *access.ClusterAccess) {
	c.Lock()
	defer c.Unlock()
	c.clusterAccesses[username] = ca
}
