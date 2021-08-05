package fake

import (
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/factory"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
)

func NewMockableApplication(crtClient kubeclient.CRTClient,
	options ...factory.Option) *MockableApplication {
	options = append(options, factory.WithServiceContextOptions(factory.CRTClientOption(crtClient)))
	return &MockableApplication{
		serviceFactory: factory.NewServiceFactory(options...),
	}
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
