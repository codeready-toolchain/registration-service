package proxy

import (
	"github.com/codeready-toolchain/registration-service/pkg/application"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/namespace"
	"github.com/gin-gonic/gin"
)

type UserNamespaces struct {
	app application.Application
}

func NewUserNamespaces(app application.Application) *UserNamespaces {
	return &UserNamespaces{
		app: app,
	}
}

func (c *UserNamespaces) GetNamespace(ctx *gin.Context, userID string) (*namespace.NamespaceAccess, error) {
	// TODO implement cache
	return c.app.MemberClusterService().GetNamespace(ctx, userID)
}
