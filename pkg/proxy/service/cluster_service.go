package service

import (
	"net/url"

	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/base"
	servicecontext "github.com/codeready-toolchain/registration-service/pkg/application/service/context"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/access"
	"github.com/codeready-toolchain/toolchain-common/pkg/cluster"

	errs "github.com/pkg/errors"
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
	signup, err := s.Services().InformerService().GetSignup(userID, username)
	if err != nil {
		return nil, err
	}
	if signup == nil || !signup.Status.Ready {
		return nil, errs.New("user is not provisioned (yet)")
	}

	// Get the target member
	members := s.GetMembersFunc()
	if len(members) == 0 {
		return nil, errs.New("no member clusters found")
	}
	for _, member := range members {
		// also check that the member cluster name matches because the api endpoint is the same for both members
		// in the e2e tests because a single cluster is used for testing multi-member scenarios
		if member.APIEndpoint == signup.APIEndpoint && member.Name == signup.ClusterName {
			apiURL, err := url.Parse(member.APIEndpoint)
			if err != nil {
				return nil, err
			}
			// requests use impersonation so are made with member ToolchainCluster token, not user tokens
			impersonatorToken := member.RestConfig.BearerToken
			return access.NewClusterAccess(*apiURL, impersonatorToken, signup.CompliantUsername), nil
		}
	}

	return nil, errs.New("no member cluster found for the user")
}
