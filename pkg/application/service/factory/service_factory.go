package factory

import (
	"fmt"

	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	servicecontext "github.com/codeready-toolchain/registration-service/pkg/application/service/context"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/pkg/namespaced"
	signupservice "github.com/codeready-toolchain/registration-service/pkg/signup/service"
	verificationservice "github.com/codeready-toolchain/registration-service/pkg/verification/service"
)

type serviceContextImpl struct {
	client   namespaced.Client
	services service.Services
}

type ServiceContextOption = func(ctx *serviceContextImpl)

func NamespacedClientOption(client namespaced.Client) ServiceContextOption {
	return func(ctx *serviceContextImpl) {
		ctx.client = client
	}
}

func (s *serviceContextImpl) Client() namespaced.Client {
	return s.client
}

func (s *serviceContextImpl) Services() service.Services {
	return s.services
}

type ServiceFactory struct {
	contextProducer       servicecontext.ServiceContextProducer
	serviceContextOptions []ServiceContextOption
	signupServiceOptions  []signupservice.SignupServiceOption
}

func (s *ServiceFactory) defaultServiceContextProducer() servicecontext.ServiceContextProducer {
	return func() servicecontext.ServiceContext {
		return &serviceContextImpl{
			services: s,
		}
	}
}

func (s *ServiceFactory) SignupService() service.SignupService {
	return signupservice.NewSignupService(s.getContext().Client(), s.signupServiceOptions...)
}

func (s *ServiceFactory) WithSignupServiceOption(opt signupservice.SignupServiceOption) {
	s.signupServiceOptions = append(s.signupServiceOptions, opt)
}

func (s *ServiceFactory) VerificationService() service.VerificationService {
	return verificationservice.NewVerificationService(s.getContext())
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

	if !configuration.IsTestingMode() {
		log.Info(nil, fmt.Sprintf("configuring a new service factory with %d options", len(options)))
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
