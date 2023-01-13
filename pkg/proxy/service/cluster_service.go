package service

import (
	"net/url"

	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/base"
	servicecontext "github.com/codeready-toolchain/registration-service/pkg/application/service/context"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/access"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/toolchain-common/pkg/cluster"

	errs "github.com/pkg/errors"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

type Option func(f *ServiceImpl)

// ServiceImpl represents the implementation of the member cluster service.
type ServiceImpl struct { // nolint:revive
	base.BaseService
	GetMembersFunc cluster.GetMemberClustersFunc
}

// NewMemberClusterService creates a service object for performing toolchain cluster related activities.
func NewMemberClusterService(context servicecontext.ServiceContext, options ...Option) service.MemberClusterService {
	si := &ServiceImpl{
		BaseService:    base.NewBaseService(context),
		GetMembersFunc: cluster.GetMemberClusters,
	}
	for _, o := range options {
		o(si)
	}
	return si
}

func (s *ServiceImpl) GetClusterAccess(userID, username string) (*access.ClusterAccess, error) {
	signup, err := s.Services().SignupService().GetSignupFromInformer(userID, username)
	if err != nil {
		return nil, err
	}
	if signup == nil || !signup.Status.Ready {
		return nil, errs.New("user is not provisioned (yet)")
	}

	return s.accessForCluster(signup.APIEndpoint, signup.ClusterName, signup.CompliantUsername)
}

func (s *ServiceImpl) GetWorkspaceAccess(userID, username, workspace string) (*access.ClusterAccess, error) {
	signup, err := s.Services().SignupService().GetSignupFromInformer(userID, username)
	if err != nil {
		return nil, err
	}
	if signup == nil || !signup.Status.Ready {
		return nil, errs.New("user is not provisioned (yet)")
	}

	space, err := s.Services().InformerService().GetSpace(workspace)
	if err != nil {
		return nil, err
	}

	return s.accessForCluster(signup.APIEndpoint, space.Status.TargetCluster, signup.CompliantUsername)
}

func (s *ServiceImpl) accessForCluster(apiEndpoint, clusterName, username string) (*access.ClusterAccess, error) {
	// Get the target member
	members := s.GetMembersFunc()
	if len(members) == 0 {
		return nil, errs.New("no member clusters found")
	}
	for _, member := range members {
		// also check that the member cluster name matches because the api endpoint is the same for both members
		// in the e2e tests because a single cluster is used for testing multi-member scenarios
		if member.APIEndpoint == apiEndpoint && member.Name == clusterName {
			apiURL, err := url.Parse(member.APIEndpoint)
			if err != nil {
				return nil, err
			}
			// requests use impersonation so are made with member ToolchainCluster token, not user tokens
			impersonatorToken := member.RestConfig.BearerToken
			return access.NewClusterAccess(*apiURL, impersonatorToken, username), nil
		}
	}

	return nil, errs.New("no member cluster found for the user")
}

type selectorOption func(labels.Selector) (labels.Selector, error)

func newSpaceBindingSelector(o ...selectorOption) (labels.Selector, error) {
	selector := labels.NewSelector()
	return selector, nil
}

func WithMurSelector(murName string) selectorOption {
	// adds label selector toolchain.dev.openshift.com/masteruserrecord: murName
	return func(selector labels.Selector) (labels.Selector, error) {
		key := toolchainv1alpha1.LabelKeyPrefix + "masteruserrecord"
		murLabel, err := labels.NewRequirement(key, selection.Equals, []string{murName})
		if err != nil {
			return nil, err
		}
		selector = selector.Add(*murLabel)
		return selector, nil
	}
}

func WithSpaceSelector(spaceName string) selectorOption {
	// adds label selector toolchain.dev.openshift.com/space: spaceName
	return func(selector labels.Selector) (labels.Selector, error) {
		key := toolchainv1alpha1.LabelKeyPrefix + "space"
		spaceLabel, err := labels.NewRequirement(key, selection.Equals, []string{spaceName})
		if err != nil {
			return nil, err
		}
		selector = selector.Add(*spaceLabel)
		return selector, nil
	}
}
