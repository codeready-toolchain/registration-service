package mock

import (
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	servicecontext "github.com/codeready-toolchain/registration-service/pkg/application/service/context"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/factory"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
)

func NewMockableApplication(config configuration.Configuration, crtClient kubeclient.CRTClient, options ...factory.Option) *MockableApplication {
	return &MockableApplication{
		serviceFactory: factory.NewServiceFactory(func() servicecontext.ServiceContext {
			return NewMockableServiceContext(crtClient, config)
		}, config, options...)}
}

type MockableApplication struct {
	serviceFactory          *factory.ServiceFactory
	mockSignupService       service.SignupService
	mockVerificationService service.VerificationService
}

func (m *MockableApplication) SignupService() service.SignupService {
	if m.mockSignupService != nil {
		return m.mockSignupService
	}
	return m.serviceFactory.SignupService()
}

func (m *MockableApplication) MockSignupService(svc service.SignupService) {
	m.mockSignupService = svc
}

func (m *MockableApplication) VerificationService() service.VerificationService {
	if m.mockVerificationService != nil {
		return m.mockVerificationService
	}
	return m.serviceFactory.VerificationService()
}

func (m *MockableApplication) MockVerificationService(svc service.VerificationService) {
	m.mockVerificationService = svc
}

type mockableServiceContext struct {
	services  service.Services
	crtClient kubeclient.CRTClient
}

func NewMockableServiceContext(crtClient kubeclient.CRTClient, config configuration.Configuration) servicecontext.ServiceContext {
	ctx := &mockableServiceContext{
		crtClient: crtClient,
	}
	var sc servicecontext.ServiceContext
	sc = ctx
	ctx.services = factory.NewServiceFactory(func() servicecontext.ServiceContext { return sc }, config)
	return sc
}

func (s *mockableServiceContext) CRTClient() kubeclient.CRTClient {
	return s.crtClient
}

func (s *mockableServiceContext) Services() service.Services {
	return s.services
}
