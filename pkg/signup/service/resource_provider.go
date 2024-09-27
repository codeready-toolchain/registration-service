package service

import (
	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
	"k8s.io/apimachinery/pkg/labels"
)

type ResourceProvider interface {
	GetMasterUserRecord(name string) (*toolchainv1alpha1.MasterUserRecord, error)
	GetToolchainStatus() (*toolchainv1alpha1.ToolchainStatus, error)
	GetUserSignup(name string) (*toolchainv1alpha1.UserSignup, error)
	GetSpace(name string) (*toolchainv1alpha1.Space, error)
	ListSpaceBindings(reqs ...labels.Requirement) ([]toolchainv1alpha1.SpaceBinding, error)
}

type CrtClientProvider struct {
	Cl kubeclient.CRTClient
}

func (p CrtClientProvider) GetMasterUserRecord(name string) (*toolchainv1alpha1.MasterUserRecord, error) {
	return p.Cl.V1Alpha1().MasterUserRecords().Get(name)
}

func (p CrtClientProvider) GetToolchainStatus() (*toolchainv1alpha1.ToolchainStatus, error) {
	return p.Cl.V1Alpha1().ToolchainStatuses().Get()
}

func (p CrtClientProvider) GetUserSignup(name string) (*toolchainv1alpha1.UserSignup, error) {
	return p.Cl.V1Alpha1().UserSignups().Get(name)
}

func (p CrtClientProvider) GetSpace(name string) (*toolchainv1alpha1.Space, error) {
	return p.Cl.V1Alpha1().Spaces().Get(name)
}

func (p CrtClientProvider) ListSpaceBindings(reqs ...labels.Requirement) ([]toolchainv1alpha1.SpaceBinding, error) {
	return p.Cl.V1Alpha1().SpaceBindings().ListSpaceBindings(reqs...)
}
