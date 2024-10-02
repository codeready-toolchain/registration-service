package context

import (
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/namespaced"
)

type ServiceContextProducer func() ServiceContext

type ServiceContext interface {
	Client() namespaced.Client
	Services() service.Services
}
