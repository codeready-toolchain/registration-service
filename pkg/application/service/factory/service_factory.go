package factory

import (
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
	service2 "github.com/codeready-toolchain/registration-service/pkg/signup/service"
	service3 "github.com/codeready-toolchain/registration-service/pkg/verification/service"
	"github.com/prometheus/common/log"

	servicecontext "github.com/codeready-toolchain/registration-service/pkg/application/service/context"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
)

type serviceContextImpl struct {
	kubeClient kubeclient.CRTClient
	services   service.Services
}

func NewServiceContext(kubeClient kubeclient.CRTClient, config *configuration.Config, options ...Option) servicecontext.ServiceContext {
	ctx := &serviceContextImpl{kubeClient: kubeClient}
	var sc servicecontext.ServiceContext
	sc = ctx
	ctx.services = NewServiceFactory(func() servicecontext.ServiceContext { return sc }, config, options...)
	return sc
}

func (s *serviceContextImpl) CRTClient() kubeclient.CRTClient {
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
	return service2.NewSignupService(s.getContext(), s.config)
}

func (s ServiceFactory) VerificationService() service.VerificationService {
	return service3.NewVerificationService(s.getContext(), s.config)
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
