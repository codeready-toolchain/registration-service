package proxy

import (
	"fmt"
	"sync"

	"github.com/codeready-toolchain/registration-service/pkg/application"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/namespace"

	"github.com/gin-gonic/gin"
)

type UserNamespaces struct {
	sync.RWMutex
	app application.Application

	// namespace accesses by username
	namespaces map[string]*namespace.NamespaceAccess
}

func NewUserNamespaces(app application.Application) *UserNamespaces {
	return &UserNamespaces{
		app:        app,
		namespaces: make(map[string]*namespace.NamespaceAccess),
	}
}

// Get tries to retrieve the namespace access from the cache first. If found then it checks if the cached access is still valid.
// If not found or invalid then retrieves a new namespace access from the member cluster service and stores it in the cache.
func (c *UserNamespaces) Get(ctx *gin.Context, userID, username string) (*namespace.NamespaceAccess, error) {
	na, err := c.namespaceFromCache(ctx, username)
	if err != nil {
		return nil, err
	}
	if na != nil {
		return na, nil
	}

	// Not found in cache or invalid. Get a new namespace access.
	log.Info(ctx, fmt.Sprintf("Retrieving a new Namespace Access from the cluster for the user : %s", username))
	na, err = c.app.MemberClusterService().GetNamespace(ctx, userID, username)
	if err != nil {
		return nil, err
	}
	// Store in the cache
	c.add(username, na)

	return na, nil
}

// namespaceFromCache tries to retrieve a namespace access from the cache and validates it.
// Returns nil, nil if no namespace access found or if it's invalid.
func (c *UserNamespaces) namespaceFromCache(ctx *gin.Context, username string) (*namespace.NamespaceAccess, error) {
	c.RLock()
	defer c.RUnlock()
	na, ok := c.namespaces[username]
	if ok {
		valid, err := na.Validate()
		if err != nil {
			return nil, err
		}
		if valid {
			log.Info(ctx, fmt.Sprintf("A valid Namespace Access found in cache for user '%s'", username))
			return na, nil
		}
		log.Info(ctx, fmt.Sprintf("Namespace Access found in cache for user '%s' but it's not valid anymore", username))
		return nil, nil
	}
	log.Info(ctx, fmt.Sprintf("Namespace Access NOT found in cache for user : %s", username))
	return nil, nil
}

func (c *UserNamespaces) add(username string, na *namespace.NamespaceAccess) {
	c.Lock()
	defer c.Unlock()
	c.namespaces[username] = na
}
