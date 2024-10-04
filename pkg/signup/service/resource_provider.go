package service

import (
	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"k8s.io/apimachinery/pkg/labels"
)

type ResourceProvider interface {
	GetMasterUserRecord(name string) (*toolchainv1alpha1.MasterUserRecord, error)
	GetToolchainStatus() (*toolchainv1alpha1.ToolchainStatus, error)
	GetUserSignup(name string) (*toolchainv1alpha1.UserSignup, error)
	GetSpace(name string) (*toolchainv1alpha1.Space, error)
	ListSpaceBindings(reqs ...labels.Requirement) ([]toolchainv1alpha1.SpaceBinding, error)
}
