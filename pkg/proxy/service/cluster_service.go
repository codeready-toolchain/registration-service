package service

import (
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/base"
	servicecontext "github.com/codeready-toolchain/registration-service/pkg/application/service/context"
)

// ServiceConfiguration represents the config used for the signup service.
type ServiceConfiguration interface {
	GetNamespace() string
}

// ServiceImpl represents the implementation of the signup service.
type ServiceImpl struct {
	base.BaseService
	Config ServiceConfiguration
}

// NewToolchainClusterService creates a service object for performing toolchain cluster related activities.
func NewToolchainClusterService(context servicecontext.ServiceContext, cfg ServiceConfiguration) service.ToolchainClusterService {
	return &ServiceImpl{
		BaseService: base.NewBaseService(context),
		Config:      cfg,
	}
}

func (s *ServiceImpl) Get(token string) error {
	//TODO
	return nil
}
