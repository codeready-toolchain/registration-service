package handlers

import (
	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// This file contains helper functions for creating a Workspace object for use in the proxy
// server and unit/e2e tests only. It is not meant for use in any operator code.
// See https://github.com/codeready-toolchain/api/pull/337 for more details.

type WorkspaceOption func(*toolchainv1alpha1.Workspace)

func NewWorkspace(name string, options ...WorkspaceOption) *toolchainv1alpha1.Workspace {
	workspace := &toolchainv1alpha1.Workspace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Workspace",
			APIVersion: "toolchain.dev.openshift.com/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	for _, option := range options {
		option(workspace)
	}
	return workspace
}

func WithNamespaces(namespaces []toolchainv1alpha1.SpaceNamespace) WorkspaceOption {
	return func(workspace *toolchainv1alpha1.Workspace) {
		workspace.Status.Namespaces = namespaces
	}
}

func WithOwner(owner string) WorkspaceOption {
	return func(workspace *toolchainv1alpha1.Workspace) {
		workspace.Status.Owner = owner
	}
}

func WithRole(role string) WorkspaceOption {
	return func(workspace *toolchainv1alpha1.Workspace) {
		workspace.Status.Role = role
	}
}
