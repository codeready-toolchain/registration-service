package proxy

import (
	"github.com/codeready-toolchain/registration-service/pkg/application"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/namespace"
)

type UserNamespaces struct {
	app application.Application
}

func NewUserNamespaces(app application.Application) *UserNamespaces {
	return &UserNamespaces{
		app: app,
	}
}

func (c *UserNamespaces) GetNamespace(userID string) (*namespace.Namespace, error) {
	// TODO implement cache
	return c.app.ToolchainClusterService().GetNamespace(userID)
}
