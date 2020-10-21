package factory

import (
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
	signup_service "github.com/codeready-toolchain/registration-service/pkg/signup/service"
	verification_service "github.com/codeready-toolchain/registration-service/pkg/verification/service"
	"github.com/prometheus/common/log"

	servicecontext "github.com/codeready-toolchain/registration-service/pkg/application/service/context"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
)

type serviceContextImpl struct {
	kubeClient kubeclient.CRTClient
	services   service.Services
}

type ServiceContextOption = func(ctx *serviceContextImpl)

func CRTClientOption(kubeClient kubeclient.CRTClient) ServiceContextOption {
	return func(ctx *serviceContextImpl) {
		ctx.kubeClient = kubeClient
	}
}

func (s *serviceContextImpl) CRTClient() kubeclient.CRTClient {
	return s.kubeClient
}

func (s *serviceContextImpl) Services() service.Services {
	return s.services
}

type ServiceFactory struct {
	contextProducer            servicecontext.ServiceContextProducer
	serviceContextOptions      []ServiceContextOption
	config                     configuration.Configuration
	verificationServiceFunc    func(opts ...verification_service.VerificationServiceOption) service.VerificationService
	verificationServiceOptions []verification_service.VerificationServiceOption
}

func (s *ServiceFactory) defaultServiceContextProducer() servicecontext.ServiceContextProducer {
	return func() servicecontext.ServiceContext {
		return &serviceContextImpl{
			services: s,
		}
	}
}

func (s *ServiceFactory) SignupService() service.SignupService {
	return signup_service.NewSignupService(s.getContext(), s.config)
}

func (s *ServiceFactory) VerificationService() service.VerificationService {
	return s.verificationServiceFunc(s.verificationServiceOptions...)
}

func (s *ServiceFactory) WithVerificationServiceOption(opt verification_service.VerificationServiceOption) {
	s.verificationServiceOptions = append(s.verificationServiceOptions, opt)
}

// Option an option to configure the Service Factory
type Option func(f *ServiceFactory)

func WithServiceContextOptions(opts ...ServiceContextOption) func(f *ServiceFactory) {
	return func(f *ServiceFactory) {
		for _, opt := range opts {
			f.serviceContextOptions = append(f.serviceContextOptions, opt)
		}
	}
}

func WithServiceContextProducer(producer servicecontext.ServiceContextProducer) func(f *ServiceFactory) {
	return func(f *ServiceFactory) {
		f.contextProducer = producer
	}
}

func NewServiceFactory(config configuration.Configuration, options ...Option) *ServiceFactory {
	f := &ServiceFactory{
		serviceContextOptions: []ServiceContextOption{},
		config:                config,
	}

	for _, opt := range options {
		opt(f)
	}

	if !config.IsTestingMode() {
		log.Info(nil, map[string]interface{}{}, "configuring a new service factory with %d options", len(options))
	}

	// default function to return an instance of Verification service
	f.verificationServiceFunc = func(opts ...verification_service.VerificationServiceOption) service.VerificationService {
		return verification_service.NewVerificationService(f.getContext(), f.config, f.verificationServiceOptions...)
	}

	return f
}

func (f *ServiceFactory) getContext() servicecontext.ServiceContext {
	var sc servicecontext.ServiceContext
	if f.contextProducer != nil {
		sc = f.contextProducer()
	} else {
		sc = f.defaultServiceContextProducer()()
	}

	for _, opt := range f.serviceContextOptions {
		if v, ok := sc.(*serviceContextImpl); ok {
			opt(v)
		}
	}

	return sc
}
