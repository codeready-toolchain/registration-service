package service

import (
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/base"
	servicecontext "github.com/codeready-toolchain/registration-service/pkg/application/service/context"
	"github.com/gin-gonic/gin"
)

// ServiceConfiguration represents the config used for the signup service.
type WoopraServiceConfiguration interface {
	GetWoopraDomain() string
}

// ServiceImpl represents the implementation of the signup service.
type WoopraServiceImpl struct {
	base.BaseService
	Config WoopraServiceConfiguration
}

// NewWoopraService creates a service object for performing user signup-related activities.
func NewWoopraService(context servicecontext.ServiceContext, cfg WoopraServiceConfiguration) service.WoopraService {
	return &WoopraServiceImpl{
		BaseService: base.NewBaseService(context),
		Config:      cfg,
	}
}

// GetWoopraDomain creates a new UserSignup resource with the specified username and userID
func (s *WoopraServiceImpl) GetWoopraDomain(ctx *gin.Context) string {
	return s.Config.GetWoopraDomain()
}
