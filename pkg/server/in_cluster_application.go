package server

import (
	"github.com/codeready-toolchain/registration-service/pkg/application"
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	servicecontext "github.com/codeready-toolchain/registration-service/pkg/application/service/context"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/factory"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
	"k8s.io/client-go/rest"
)

func NewInClusterApplication(config configuration.Configuration, options ...factory.Option) (application.Application, error) {
	app := new(InClusterApplication)

	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	kubeClient, err := kubeclient.NewCRTRESTClient(k8sConfig, config.GetNamespace())
	if err != nil {
		return nil, err
	}

	app.serviceFactory = factory.NewServiceFactory(func() servicecontext.ServiceContext {
		return factory.NewServiceContext(kubeClient, config)
	}, config, options...)
	return app, nil
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
