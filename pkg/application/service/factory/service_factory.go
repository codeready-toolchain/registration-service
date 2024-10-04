package factory

import (
	"fmt"

	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	servicecontext "github.com/codeready-toolchain/registration-service/pkg/application/service/context"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	informerservice "github.com/codeready-toolchain/registration-service/pkg/informers/service"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/pkg/namespaced"
	clusterservice "github.com/codeready-toolchain/registration-service/pkg/proxy/service"
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
	contextProducer            servicecontext.ServiceContextProducer
	serviceContextOptions      []ServiceContextOption
	verificationServiceFunc    func(opts ...verificationservice.VerificationServiceOption) service.VerificationService
	verificationServiceOptions []verificationservice.VerificationServiceOption
	signupServiceFunc          func(opts ...signupservice.SignupServiceOption) service.SignupService
	signupServiceOptions       []signupservice.SignupServiceOption
}

func (s *ServiceFactory) defaultServiceContextProducer() servicecontext.ServiceContextProducer {
	return func() servicecontext.ServiceContext {
		return &serviceContextImpl{
			services: s,
		}
	}
}

func (s *ServiceFactory) InformerService() service.InformerService {
	return informerservice.NewInformerService(s.getContext().Client(), configuration.Namespace())
}

func (s *ServiceFactory) MemberClusterService() service.MemberClusterService {
	return clusterservice.NewMemberClusterService(s.getContext())
}

func (s *ServiceFactory) SignupService() service.SignupService {
	return s.signupServiceFunc(s.signupServiceOptions...)
}

func (s *ServiceFactory) WithSignupServiceOption(opt signupservice.SignupServiceOption) {
	s.signupServiceOptions = append(s.signupServiceOptions, opt)
}

func (s *ServiceFactory) VerificationService() service.VerificationService {
	return s.verificationServiceFunc(s.verificationServiceOptions...)
}

func (s *ServiceFactory) WithVerificationServiceOption(opt verificationservice.VerificationServiceOption) {
	s.verificationServiceOptions = append(s.verificationServiceOptions, opt)
}

func (s *ServiceFactory) WithSignupService(signupService service.SignupService) {
	s.signupServiceFunc = func(_ ...signupservice.SignupServiceOption) service.SignupService {
		return signupService
	}
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

	// default function to return an instance of Verification service
	f.verificationServiceFunc = func(_ ...verificationservice.VerificationServiceOption) service.VerificationService {
		return verificationservice.NewVerificationService(f.getContext(), f.verificationServiceOptions...)
	}

	if f.signupServiceFunc == nil {
		f.signupServiceFunc = func(_ ...signupservice.SignupServiceOption) service.SignupService {
			return signupservice.NewSignupService(f.getContext(), f.signupServiceOptions...)
		}
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
