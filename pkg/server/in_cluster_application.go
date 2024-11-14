package server

import (
	"github.com/codeready-toolchain/registration-service/pkg/application"
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/factory"
	"github.com/codeready-toolchain/registration-service/pkg/namespaced"
)

// NewInClusterApplication creates a new in-cluster application with the specified configuration and options.  This
// application type is intended to run inside a Kubernetes cluster, where it makes use of the rest.InClusterConfig()
// function to determine which Kubernetes configuration to use to create the REST client that interacts with the
// Kubernetes service endpoints.
func NewInClusterApplication(client namespaced.Client, options ...factory.Option) application.Application {
	return &InClusterApplication{
		serviceFactory: factory.NewServiceFactory(append(options,
			factory.WithServiceContextOptions(
				factory.NamespacedClientOption(client)))...),
	}
}

type InClusterApplication struct {
	serviceFactory *factory.ServiceFactory
}

func (r InClusterApplication) SignupService() service.SignupService {
	return r.serviceFactory.SignupService()
}

func (r InClusterApplication) VerificationService() service.VerificationService {
	return r.serviceFactory.VerificationService()
}
