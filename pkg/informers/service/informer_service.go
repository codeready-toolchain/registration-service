package service

import (
	"context"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/toolchain-common/pkg/hash"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Option func(f *ServiceImpl)

// ServiceImpl represents the implementation of the informer service.
type ServiceImpl struct { // nolint:revive
	client    client.Client
	namespace string
}

// NewInformerService creates a service object for getting resources
func NewInformerService(client client.Client, namespace string) service.InformerService {
	si := &ServiceImpl{
		client:    client,
		namespace: namespace,
	}
	return si
}

func (s *ServiceImpl) GetProxyPluginConfig(name string) (*toolchainv1alpha1.ProxyPlugin, error) {
	pluginConfig := &toolchainv1alpha1.ProxyPlugin{}
	namespacedName := types.NamespacedName{Name: name, Namespace: s.namespace}
	err := s.client.Get(context.TODO(), namespacedName, pluginConfig)
	return pluginConfig, err
}

func (s *ServiceImpl) GetMasterUserRecord(name string) (*toolchainv1alpha1.MasterUserRecord, error) {
	mur := &toolchainv1alpha1.MasterUserRecord{}
	namespacedName := types.NamespacedName{Name: name, Namespace: s.namespace}
	err := s.client.Get(context.TODO(), namespacedName, mur)
	return mur, err
}

func (s *ServiceImpl) GetSpace(name string) (*toolchainv1alpha1.Space, error) {
	space := &toolchainv1alpha1.Space{}
	namespacedName := types.NamespacedName{Name: name, Namespace: s.namespace}
	err := s.client.Get(context.TODO(), namespacedName, space)
	return space, err
}

func (s *ServiceImpl) GetToolchainStatus() (*toolchainv1alpha1.ToolchainStatus, error) {
	status := &toolchainv1alpha1.ToolchainStatus{}
	namespacedName := types.NamespacedName{Name: "toolchain-status", Namespace: s.namespace}
	err := s.client.Get(context.TODO(), namespacedName, status)
	return status, err
}

func (s *ServiceImpl) GetUserSignup(name string) (*toolchainv1alpha1.UserSignup, error) {
	signup := &toolchainv1alpha1.UserSignup{}
	namespacedName := types.NamespacedName{Name: name, Namespace: s.namespace}
	err := s.client.Get(context.TODO(), namespacedName, signup)
	return signup, err
}

func (s *ServiceImpl) ListSpaceBindings(reqs ...labels.Requirement) ([]toolchainv1alpha1.SpaceBinding, error) {
	selector := labels.NewSelector()
	for i := range reqs {
		selector = selector.Add(reqs[i])
	}

	bindings := &toolchainv1alpha1.SpaceBindingList{}
	if err := s.client.List(context.TODO(), bindings, client.InNamespace(s.namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return nil, err
	}
	return bindings.Items, nil
}

func (s *ServiceImpl) GetNSTemplateTier(name string) (*toolchainv1alpha1.NSTemplateTier, error) {
	tier := &toolchainv1alpha1.NSTemplateTier{}
	namespacedName := types.NamespacedName{Name: name, Namespace: s.namespace}
	err := s.client.Get(context.TODO(), namespacedName, tier)
	return tier, err
}

func (s *ServiceImpl) ListBannedUsersByEmail(email string) ([]toolchainv1alpha1.BannedUser, error) {
	hashedEmail := hash.EncodeString(email)

	bannedUsers := &toolchainv1alpha1.BannedUserList{}
	if err := s.client.List(context.TODO(), bannedUsers, client.InNamespace(s.namespace),
		client.MatchingLabels{toolchainv1alpha1.BannedUserEmailHashLabelKey: hashedEmail}); err != nil {
		return nil, err
	}

	return bannedUsers.Items, nil
}
