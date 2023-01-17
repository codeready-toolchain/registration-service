package fake

import (
	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
)

func NewFakeInformer() *Informer {
	return &Informer{}
}

type Informer struct {
	GetMurFunc             func(name string) (*toolchainv1alpha1.MasterUserRecord, error)
	GetToolchainStatusFunc func() (*toolchainv1alpha1.ToolchainStatus, error)
	GetUserSignupFunc      func(name string) (*toolchainv1alpha1.UserSignup, error)
}

func (f Informer) GetMasterUserRecord(name string) (*toolchainv1alpha1.MasterUserRecord, error) {
	if f.GetMurFunc != nil {
		return f.GetMurFunc(name)
	}
	panic("not supposed to call GetMasterUserRecord")
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
