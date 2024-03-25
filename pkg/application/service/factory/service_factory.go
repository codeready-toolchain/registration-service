package factory

import (
	"fmt"

	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	servicecontext "github.com/codeready-toolchain/registration-service/pkg/application/service/context"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/informers"
	informerservice "github.com/codeready-toolchain/registration-service/pkg/informers/service"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	clusterservice "github.com/codeready-toolchain/registration-service/pkg/proxy/service"
	signupservice "github.com/codeready-toolchain/registration-service/pkg/signup/service"
	verificationservice "github.com/codeready-toolchain/registration-service/pkg/verification/service"
	commonconfig "github.com/codeready-toolchain/toolchain-common/pkg/configuration"
)

type serviceContextImpl struct {
	kubeClient kubeclient.CRTClient
	informer   informers.Informer
	services   service.Services
}

type ServiceContextOption = func(ctx *serviceContextImpl)

func CRTClientOption(kubeClient kubeclient.CRTClient) ServiceContextOption {
	return func(ctx *serviceContextImpl) {
		ctx.kubeClient = kubeClient
	}
}

func InformerOption(informer informers.Informer) ServiceContextOption {
	return func(ctx *serviceContextImpl) {
		ctx.informer = informer
	}
}

func (s *serviceContextImpl) Informer() informers.Informer {
	return s.informer
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
	verificationServiceFunc    func(opts ...verificationservice.VerificationServiceOption) service.VerificationService
	verificationServiceOptions []verificationservice.VerificationServiceOption
	signupServiceFunc          func(opts ...signupservice.SignupServiceOption) service.SignupService
	signupServiceOptions       []signupservice.SignupServiceOption
	publicViewerConfig         commonconfig.PublicViewerConfig
}

func (s *ServiceFactory) defaultServiceContextProducer() servicecontext.ServiceContextProducer {
	return func() servicecontext.ServiceContext {
		return &serviceContextImpl{
			services: s,
		}
	}
}

func (s *ServiceFactory) InformerService() service.InformerService {
	return informerservice.NewInformerService(s.getContext())
}

func (s *ServiceFactory) MemberClusterService() service.MemberClusterService {
	return clusterservice.NewMemberClusterService(s.getContext(), clusterservice.WithPublicViewerConfig(s.publicViewerConfig))
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

// Option an option to configure the Service Factory
type Option func(f *ServiceFactory)

func WithServiceContextOptions(opts ...ServiceContextOption) func(f *ServiceFactory) {
	return func(f *ServiceFactory) {
		f.serviceContextOptions = append(f.serviceContextOptions, opts...)
	}
}

func WithPublicViewerConfig(config commonconfig.PublicViewerConfig) func(f *ServiceFactory) {
	return func(f *ServiceFactory) {
		f.publicViewerConfig = config
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

	f.signupServiceFunc = func(_ ...signupservice.SignupServiceOption) service.SignupService {
		return signupservice.NewSignupService(f.getContext(), f.signupServiceOptions...)
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
