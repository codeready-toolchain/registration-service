package service

import (
	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
	"k8s.io/apimachinery/pkg/labels"
)

type ResourceProvider interface {
	GetMasterUserRecord(name string) (*toolchainv1alpha1.MasterUserRecord, error)
	GetUserTier(name string) (*toolchainv1alpha1.UserTier, error)
	GetToolchainStatus() (*toolchainv1alpha1.ToolchainStatus, error)
	GetUserSignup(name string) (*toolchainv1alpha1.UserSignup, error)
	GetSpace(name string) (*toolchainv1alpha1.Space, error)
	ListSpaceBindings(reqs ...labels.Requirement) ([]toolchainv1alpha1.SpaceBinding, error)
}

type crtClientProvider struct {
	cl kubeclient.CRTClient
}

func (p crtClientProvider) GetMasterUserRecord(name string) (*toolchainv1alpha1.MasterUserRecord, error) {
	return p.cl.V1Alpha1().MasterUserRecords().Get(name)
}

func (p crtClientProvider) GetUserTier(name string) (*toolchainv1alpha1.UserTier, error) {
	return p.cl.V1Alpha1().UserTiers().Get(name)
}

func (p crtClientProvider) GetToolchainStatus() (*toolchainv1alpha1.ToolchainStatus, error) {
	return p.cl.V1Alpha1().ToolchainStatuses().Get()
}

func (p crtClientProvider) GetUserSignup(name string) (*toolchainv1alpha1.UserSignup, error) {
	return p.cl.V1Alpha1().UserSignups().Get(name)
}

func (p crtClientProvider) GetSpace(name string) (*toolchainv1alpha1.Space, error) {
	return p.cl.V1Alpha1().Spaces().Get(name)
}

func (p crtClientProvider) ListSpaceBindings(reqs ...labels.Requirement) ([]toolchainv1alpha1.SpaceBinding, error) {
	return p.cl.V1Alpha1().SpaceBindings().ListSpaceBindings(reqs...)
}
