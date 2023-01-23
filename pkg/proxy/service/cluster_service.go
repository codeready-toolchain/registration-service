package service

import (
	"fmt"
	"net/url"

	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/base"
	servicecontext "github.com/codeready-toolchain/registration-service/pkg/application/service/context"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/access"
	"github.com/gin-gonic/gin"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
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

func (s *ServiceImpl) GetClusterAccess(ctx *gin.Context, userID, username, workspace string) (*access.ClusterAccess, error) {
	signup, err := s.Services().SignupService().GetSignupFromInformer(userID, username)
	if err != nil {
		return nil, err
	}
	if signup == nil || !signup.Status.Ready {
		return nil, errs.New("user is not provisioned (yet)")
	}

	// if workspace is not provided then return the default space access
	if workspace == "" {
		return s.accessForCluster(signup.APIEndpoint, signup.ClusterName, signup.CompliantUsername)
	}

	// look up space
	space, err := s.Services().InformerService().GetSpace(workspace)
	if err != nil {
		// log the actual error but do not return it so that it doesn't reveal information about a space that may not belong to the requestor
		log.Error(ctx, err, "unable to get target cluster")
		return nil, fmt.Errorf("the requested space in not available")
	}

	return s.accessForSpace(ctx, space, signup.CompliantUsername)
}

func (s *ServiceImpl) accessForSpace(ctx *gin.Context, space *toolchainv1alpha1.Space, username string) (*access.ClusterAccess, error) {
	// Get the target member
	members := s.GetMembersFunc()
	if len(members) == 0 {
		return nil, errs.New("no member clusters found")
	}
	for _, member := range members {
		if member.Name == space.Status.TargetCluster {
			apiURL, err := url.Parse(member.APIEndpoint)
			if err != nil {
				return nil, err
			}
			// requests use impersonation so are made with member ToolchainCluster token, not user tokens
			impersonatorToken := member.RestConfig.BearerToken
			return access.NewClusterAccess(*apiURL, impersonatorToken, username), nil
		}
	}

	errMsg := fmt.Sprintf("no member cluster found for space '%s'", space.Name)
	log.Error(ctx, fmt.Errorf("no matching target cluster '%s' for the space", space.Status.TargetCluster), errMsg)
	return nil, errs.New(errMsg)
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
