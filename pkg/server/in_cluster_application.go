package server

import (
	"github.com/codeready-toolchain/registration-service/pkg/application"
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/namespaced"
	signupservice "github.com/codeready-toolchain/registration-service/pkg/signup/service"
	verificationservice "github.com/codeready-toolchain/registration-service/pkg/verification/service"
)

// NewInClusterApplication creates a new in-cluster application.
func NewInClusterApplication(client namespaced.Client) application.Application {
	return &InClusterApplication{
		signupService:       signupservice.NewSignupService(client),
		verificationService: verificationservice.NewVerificationService(client),
	}
}

type InClusterApplication struct {
	signupService       service.SignupService
	verificationService service.VerificationService
}

func (r InClusterApplication) SignupService() service.SignupService {
	return r.signupService
}

func (r InClusterApplication) VerificationService() service.VerificationService {
	return r.verificationService
}
