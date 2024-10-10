package fake

import (
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/factory"
)

func NewMockableApplication(options ...factory.Option) *MockableApplication {
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

func (m *MockableApplication) MemberClusterService() service.MemberClusterService {
	return m.serviceFactory.MemberClusterService()
}
