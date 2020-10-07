package server

import (
	"github.com/codeready-toolchain/registration-service/pkg/application"
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	servicecontext "github.com/codeready-toolchain/registration-service/pkg/application/service/context"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/factory"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
)

func NewMockableApplication(config *configuration.Config, options ...factory.Option) (application.Application, error) {
	app := new(MockableApplication)

	app.serviceFactory = factory.NewServiceFactory(func() servicecontext.ServiceContext {
		return NewMockableServiceContext(nil, config)
	}, config, options...)
	return app, nil
}

type MockableApplication struct {
	serviceFactory *factory.ServiceFactory
}

func (r MockableApplication) SignupService() service.SignupService {
	return r.serviceFactory.SignupService()
}

func (r MockableApplication) VerificationService() service.VerificationService {
	return r.serviceFactory.VerificationService()
}

type mockableServiceContext struct {
	services   service.Services
	mockClient kubeclient.CRTClient
}

func NewMockableServiceContext(mockClient kubeclient.CRTClient, config *configuration.Config) servicecontext.ServiceContext {
	ctx := &mockableServiceContext{
		mockClient: mockClient,
	}
	var sc servicecontext.ServiceContext
	sc = ctx
	ctx.services = factory.NewServiceFactory(func() servicecontext.ServiceContext { return sc }, config)
	return sc
}

func (s *mockableServiceContext) CRTClient() kubeclient.CRTClient {
	return s.mockClient
}

func (s *mockableServiceContext) Services() service.Services {
	return s.services
}
