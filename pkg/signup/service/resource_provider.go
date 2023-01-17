package service

import (
	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
)

type ResourceProvider interface {
	GetMasterUserRecord(name string) (*toolchainv1alpha1.MasterUserRecord, error)
	GetToolchainStatus() (*toolchainv1alpha1.ToolchainStatus, error)
	GetUserSignup(name string) (*toolchainv1alpha1.UserSignup, error)
}

type crtClientProvider struct {
	cl kubeclient.CRTClient
}

func (p crtClientProvider) GetMasterUserRecord(name string) (*toolchainv1alpha1.MasterUserRecord, error) {
	return p.cl.V1Alpha1().MasterUserRecords().Get(name)
}

func (p crtClientProvider) GetToolchainStatus() (*toolchainv1alpha1.ToolchainStatus, error) {
	return p.cl.V1Alpha1().ToolchainStatuses().Get()
}

func (p crtClientProvider) GetUserSignup(name string) (*toolchainv1alpha1.UserSignup, error) {
	return p.cl.V1Alpha1().UserSignups().Get(name)
}
