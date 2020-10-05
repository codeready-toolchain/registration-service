package factory

import (
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/registration-service/pkg/verification"
	"github.com/prometheus/common/log"

	servicecontext "github.com/codeready-toolchain/registration-service/pkg/application/service/context"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
)

type serviceContextImpl struct {
	kubeClient kubeclient.CRTV1Alpha1Client
	services   service.Services
}

func NewServiceContext(kubeClient kubeclient.CRTV1Alpha1Client, config *configuration.Config, options ...Option) servicecontext.ServiceContext {
	ctx := &serviceContextImpl{kubeClient: kubeClient}
	var sc servicecontext.ServiceContext
	sc = ctx
	ctx.services = NewServiceFactory(func() servicecontext.ServiceContext { return sc }, config, options...)
	return sc
}

func (s *serviceContextImpl) CRTV1Alpha1Client() kubeclient.CRTV1Alpha1Client {
	return s.kubeClient
}

func (s *serviceContextImpl) Services() service.Services {
	return s.services
}

type ServiceFactory struct {
	contextProducer servicecontext.ServiceContextProducer
	config          *configuration.Config
}

func (s ServiceFactory) SignupService() service.SignupService {
	return signup.NewSignupService(s.getContext(), s.config)
}

func (s ServiceFactory) VerificationService() service.VerificationService {
	return verification.NewVerificationService(s.getContext(), s.config)
}

// Option an option to configure the Service Factory
type Option func(f *ServiceFactory)

func NewServiceFactory(producer servicecontext.ServiceContextProducer, config *configuration.Config, options ...Option) *ServiceFactory {
	f := &ServiceFactory{contextProducer: producer, config: config}

	log.Info(nil, map[string]interface{}{}, "configuring a new service factory with %d options", len(options))
	// and options
	for _, opt := range options {
		opt(f)
	}
	return f
}

func (f *ServiceFactory) getContext() servicecontext.ServiceContext {
	return f.contextProducer()
}
