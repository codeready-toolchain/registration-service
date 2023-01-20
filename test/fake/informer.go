package fake

import (
	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewFakeInformer() *Informer {
	return &Informer{}
}

type Informer struct {
	GetMurFunc             func(name string) (*toolchainv1alpha1.MasterUserRecord, error)
	GetSpaceFunc           func(name string) (*toolchainv1alpha1.Space, error)
	GetToolchainStatusFunc func() (*toolchainv1alpha1.ToolchainStatus, error)
	GetUserSignupFunc      func(name string) (*toolchainv1alpha1.UserSignup, error)
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

func NewSpace(targetCluster, compliantUserName string) *toolchainv1alpha1.Space {
	space := &toolchainv1alpha1.Space{
		ObjectMeta: metav1.ObjectMeta{
			Name: compliantUserName,
		},
		Spec: toolchainv1alpha1.SpaceSpec{
			TargetCluster: targetCluster,
			TierName:      "base1ns",
		},
		Status: toolchainv1alpha1.SpaceStatus{
			TargetCluster: targetCluster,
		},
	}
	return space
}
