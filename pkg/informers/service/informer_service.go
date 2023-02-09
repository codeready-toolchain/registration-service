package service

import (
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/base"
	servicecontext "github.com/codeready-toolchain/registration-service/pkg/application/service/context"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/informers"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient/resources"
	"github.com/codeready-toolchain/registration-service/pkg/log"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
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

func (s *ServiceImpl) GetSpace(name string) (*toolchainv1alpha1.Space, error) {
	obj, err := s.informer.Space.ByNamespace(configuration.Namespace()).Get(name)
	if err != nil {
		return nil, err
	}

	unobj := obj.(*unstructured.Unstructured)
	space := &toolchainv1alpha1.Space{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unobj.UnstructuredContent(), space); err != nil {
		log.Errorf(nil, err, "failed to get Space '%s'", name)
		return nil, err
	}
	return space, err
}

func (s *ServiceImpl) GetToolchainStatus() (*toolchainv1alpha1.ToolchainStatus, error) {
	obj, err := s.informer.ToolchainStatus.ByNamespace(configuration.Namespace()).Get(resources.ToolchainStatusName)
	if err != nil {
		return nil, err
	}

	unobj := obj.(*unstructured.Unstructured)
	stat := &toolchainv1alpha1.ToolchainStatus{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unobj.UnstructuredContent(), stat); err != nil {
		log.Errorf(nil, err, "failed to get ToolchainStatus %s", resources.ToolchainStatusName)
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

func (s *ServiceImpl) ListSpaceBindings(reqs ...labels.Requirement) ([]*toolchainv1alpha1.SpaceBinding, error) {
	selector := labels.NewSelector().Add(reqs...)
	objs, err := s.informer.SpaceBinding.ByNamespace(configuration.Namespace()).List(selector)
	if err != nil {
		return nil, err
	}

	sbs := []*toolchainv1alpha1.SpaceBinding{}
	for _, obj := range objs {
		unobj := obj.(*unstructured.Unstructured)
		sb := &toolchainv1alpha1.SpaceBinding{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unobj.UnstructuredContent(), sb); err != nil {
			log.Errorf(nil, err, "failed to list SpaceBindings")
			return nil, err
		}
		sbs = append(sbs, sb)
	}
	return sbs, err
}
