package fake

import (
	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func NewFakeInformer() Informer {
	return Informer{}
}

type Informer struct {
	GetMurFunc               func(name string) (*toolchainv1alpha1.MasterUserRecord, error)
	GetSpaceFunc             func(name string) (*toolchainv1alpha1.Space, error)
	GetToolchainStatusFunc   func() (*toolchainv1alpha1.ToolchainStatus, error)
	GetUserSignupFunc        func(name string) (*toolchainv1alpha1.UserSignup, error)
	ListSpaceBindingFunc     func(reqs ...labels.Requirement) ([]toolchainv1alpha1.SpaceBinding, error)
	GetProxyPluginConfigFunc func(name string) (*toolchainv1alpha1.ProxyPlugin, error)
	GetNSTemplateTierFunc    func(name string) (*toolchainv1alpha1.NSTemplateTier, error)
}

func (f Informer) GetProxyPluginConfig(name string) (*toolchainv1alpha1.ProxyPlugin, error) {
	if f.GetProxyPluginConfigFunc != nil {
		return f.GetProxyPluginConfigFunc(name)
	}
	panic("not supposed to call GetProxyPluginConfig")
}

func (f Informer) GetMasterUserRecord(name string) (*toolchainv1alpha1.MasterUserRecord, error) {
	if f.GetMurFunc != nil {
		return f.GetMurFunc(name)
	}
	panic("not supposed to call GetMasterUserRecord")
}

func (f Informer) GetSpace(name string) (*toolchainv1alpha1.Space, error) {
	if f.GetSpaceFunc != nil {
		return f.GetSpaceFunc(name)
	}
	panic("not supposed to call GetSpace")
}

func (f Informer) GetToolchainStatus() (*toolchainv1alpha1.ToolchainStatus, error) {
	if f.GetToolchainStatusFunc != nil {
		return f.GetToolchainStatusFunc()
	}
	panic("not supposed to call GetToolchainStatus")
}

func (f Informer) GetUserSignup(name string) (*toolchainv1alpha1.UserSignup, error) {
	if f.GetUserSignupFunc != nil {
		return f.GetUserSignupFunc(name)
	}
	panic("not supposed to call GetUserSignup")
}

func (f Informer) ListSpaceBindings(req ...labels.Requirement) ([]toolchainv1alpha1.SpaceBinding, error) {
	if f.ListSpaceBindingFunc != nil {
		return f.ListSpaceBindingFunc(req...)
	}
	panic("not supposed to call ListSpaceBindings")
}

func (f Informer) GetNSTemplateTier(tier string) (*toolchainv1alpha1.NSTemplateTier, error) {
	if f.GetNSTemplateTierFunc != nil {
		return f.GetNSTemplateTierFunc(tier)
	}
	panic("not supposed to call GetNSTemplateTierFunc")
}

func NewSpace(name, targetCluster, compliantUserName string) *toolchainv1alpha1.Space {
	space := &toolchainv1alpha1.Space{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: configuration.Namespace(),
			Labels: map[string]string{
				toolchainv1alpha1.SpaceCreatorLabelKey: compliantUserName,
			},
		},
		Spec: toolchainv1alpha1.SpaceSpec{
			TargetCluster: targetCluster,
			TierName:      "base1ns",
		},
		Status: toolchainv1alpha1.SpaceStatus{
			TargetCluster: targetCluster,
			ProvisionedNamespaces: []toolchainv1alpha1.SpaceNamespace{
				{
					Name: "john-dev",
					Type: "default",
				},
				{
					Name: "john-stage",
				},
			},
		},
	}
	return space
}

func NewSpaceBinding(name, murLabelValue, spaceLabelValue, role string) *toolchainv1alpha1.SpaceBinding {
	return &toolchainv1alpha1.SpaceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				toolchainv1alpha1.SpaceBindingMasterUserRecordLabelKey: murLabelValue,
				toolchainv1alpha1.SpaceBindingSpaceLabelKey:            spaceLabelValue,
			},
		},
		Spec: toolchainv1alpha1.SpaceBindingSpec{
			SpaceRole: role,
		},
	}
}
