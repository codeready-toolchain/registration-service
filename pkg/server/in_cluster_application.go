package server

import (
	"github.com/codeready-toolchain/registration-service/pkg/application"
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/factory"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/informers"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
	commonconfig "github.com/codeready-toolchain/toolchain-common/pkg/configuration"
	"k8s.io/client-go/rest"
)

// NewInClusterApplication creates a new in-cluster application with the specified configuration and options.  This
// application type is intended to run inside a Kubernetes cluster, where it makes use of the rest.InClusterConfig()
// function to determine which Kubernetes configuration to use to create the REST client that interacts with the
// Kubernetes service endpoints.
func NewInClusterApplication(informer informers.Informer, config commonconfig.PublicViewerConfig) (application.Application, error) {
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	kubeClient, err := kubeclient.NewCRTRESTClient(k8sConfig, informer, configuration.Namespace())
	if err != nil {
		return nil, err
	}

	return &InClusterApplication{
		serviceFactory: factory.NewServiceFactory(
			factory.WithServiceContextOptions(factory.CRTClientOption(kubeClient),
				factory.InformerOption(informer),
			),
			factory.WithPublicViewerConfig(config),
		),
	}, nil
}

type InClusterApplication struct {
	serviceFactory *factory.ServiceFactory
}

func (r InClusterApplication) InformerService() service.InformerService {
	return r.serviceFactory.InformerService()
}

func (r InClusterApplication) SignupService() service.SignupService {
	return r.serviceFactory.SignupService()
}

func (r InClusterApplication) VerificationService() service.VerificationService {
	return r.serviceFactory.VerificationService()
}

func (r InClusterApplication) MemberClusterService() service.MemberClusterService {
	return r.serviceFactory.MemberClusterService()
}
