package factory

import (
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
	signup_service "github.com/codeready-toolchain/registration-service/pkg/signup/service"
	verification_service "github.com/codeready-toolchain/registration-service/pkg/verification/service"
	"github.com/prometheus/common/log"

	servicecontext "github.com/codeready-toolchain/registration-service/pkg/application/service/context"
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
	return signup_service.NewSignupService(s.getContext())
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
		f.serviceContextOptions = append(f.serviceContextOptions, opts...)
	}
}

func NewServiceFactory(options ...Option) *ServiceFactory {
	f := &ServiceFactory{
		serviceContextOptions: []ServiceContextOption{},
	}

	for _, opt := range options {
		opt(f)
	}

	if configuration.IsTestingMode() {
		log.Info(nil, map[string]interface{}{}, "configuring a new service factory with %d options", len(options))
	}

	// default function to return an instance of Verification service
	f.verificationServiceFunc = func(opts ...verification_service.VerificationServiceOption) service.VerificationService {
		return verification_service.NewVerificationService(f.getContext(), f.verificationServiceOptions...)
	}

	return f
}

func (s *ServiceFactory) getContext() servicecontext.ServiceContext {
	var sc servicecontext.ServiceContext
	if s.contextProducer != nil {
		sc = s.contextProducer()
	} else {
		sc = s.defaultServiceContextProducer()()
	}

	for _, opt := range s.serviceContextOptions {
		if v, ok := sc.(*serviceContextImpl); ok {
			opt(v)
		}
	}

	return sc
}
