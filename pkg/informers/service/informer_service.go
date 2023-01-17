package service

import (
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/base"
	servicecontext "github.com/codeready-toolchain/registration-service/pkg/application/service/context"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/informers"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
	"github.com/codeready-toolchain/registration-service/pkg/log"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type Option func(f *ServiceImpl)

// ServiceImpl represents the implementation of the informer service.
type ServiceImpl struct { // nolint:revive
	base.BaseService
	informer informers.Informer
}

// NewInformerService creates a service object for getting resources via shared informers
func NewInformerService(context servicecontext.ServiceContext, options ...Option) service.InformerService {
	si := &ServiceImpl{
		BaseService: base.NewBaseService(context),
		informer:    context.Informer(),
	}
	for _, o := range options {
		o(si)
	}
	return si
}

func (s *ServiceImpl) GetMasterUserRecord(name string) (*toolchainv1alpha1.MasterUserRecord, error) {
	obj, err := s.informer.Masteruserrecord.ByNamespace(configuration.Namespace()).Get(name)
	if err != nil {
		return nil, err
	}

	unobj := obj.(*unstructured.Unstructured)
	mur := &toolchainv1alpha1.MasterUserRecord{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unobj.UnstructuredContent(), mur); err != nil {
		log.Errorf(nil, err, "failed to get MasterUserRecord '%s'", name)
		return nil, err
	}
	return mur, err
}

func (s *ServiceImpl) GetToolchainStatus() (*toolchainv1alpha1.ToolchainStatus, error) {
	obj, err := s.informer.ToolchainStatus.ByNamespace(configuration.Namespace()).Get(kubeclient.ToolchainStatusName)
	if err != nil {
		return nil, err
	}

	unobj := obj.(*unstructured.Unstructured)
	stat := &toolchainv1alpha1.ToolchainStatus{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unobj.UnstructuredContent(), stat); err != nil {
		log.Errorf(nil, err, "failed to get ToolchainStatus %s", kubeclient.ToolchainStatusName)
		return nil, err
	}
	return stat, err
}

func (s *ServiceImpl) GetUserSignup(name string) (*toolchainv1alpha1.UserSignup, error) {
	obj, err := s.informer.UserSignup.ByNamespace(configuration.Namespace()).Get(name)
	if err != nil {
		return nil, err
	}

	unobj := obj.(*unstructured.Unstructured)
	us := &toolchainv1alpha1.UserSignup{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unobj.UnstructuredContent(), us); err != nil {
		log.Errorf(nil, err, "failed to get UserSignup '%s'", name)
		return nil, err
	}
	return us, err
}
