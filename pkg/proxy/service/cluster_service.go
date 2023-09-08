package service

import (
	"context"
	"fmt"
	"net/url"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/base"
	servicecontext "github.com/codeready-toolchain/registration-service/pkg/application/service/context"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/access"
	"github.com/codeready-toolchain/toolchain-common/pkg/cluster"

	routev1 "github.com/openshift/api/route/v1"

	errs "github.com/pkg/errors"

	"k8s.io/apimachinery/pkg/types"
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

func (s *ServiceImpl) GetClusterAccess(userID, username, workspace, proxyPluginName string) (*access.ClusterAccess, error) {
	signup, err := s.Services().SignupService().GetSignupFromInformer(nil, userID, username, false) // don't check for usersignup complete status, since it might cause issues with proxy calls when quick transitions from ready to provisioning are happening.
	if err != nil {
		return nil, err
	}
	// if signup has the CompliantUsername set it means that MUR was created and useraccount is provisioned
	if signup == nil || signup.CompliantUsername == "" {
		cause := errs.New("user is not provisioned (yet)")
		log.Error(nil, cause, fmt.Sprintf("signup object: %+v", signup))
		return nil, cause
	}

	// if workspace is not provided then return the default space access
	if workspace == "" {
		return s.accessForCluster(signup.APIEndpoint, signup.ClusterName, signup.CompliantUsername, proxyPluginName)
	}

	// look up space
	space, err := s.Services().InformerService().GetSpace(workspace)
	if err != nil {
		// log the actual error but do not return it so that it doesn't reveal information about a space that may not belong to the requestor
		log.Error(nil, err, "unable to get target cluster for workspace "+workspace)
		return nil, fmt.Errorf("the requested space is not available")
	}

	return s.accessForSpace(space, signup.CompliantUsername, proxyPluginName)
}

func (s *ServiceImpl) accessForSpace(space *toolchainv1alpha1.Space, username, proxyPluginName string) (*access.ClusterAccess, error) {
	// Get the target member
	members := s.GetMembersFunc()
	if len(members) == 0 {
		return nil, errs.New("no member clusters found")
	}
	for _, member := range members {
		if member.Name == space.Status.TargetCluster {
			apiURL, err := s.getMemberURL(proxyPluginName, member)
			if err != nil {
				return nil, err
			}
			// requests use impersonation so are made with member ToolchainCluster token, not user tokens
			impersonatorToken := member.RestConfig.BearerToken
			return access.NewClusterAccess(*apiURL, impersonatorToken, username), nil
		}
	}

	errMsg := fmt.Sprintf("no member cluster found for space '%s'", space.Name)
	log.Error(nil, fmt.Errorf("no matching target cluster '%s' for the space", space.Status.TargetCluster), errMsg)
	return nil, errs.New(errMsg)
}

func (s *ServiceImpl) accessForCluster(apiEndpoint, clusterName, username, proxyPluginName string) (*access.ClusterAccess, error) {
	// Get the target member
	members := s.GetMembersFunc()
	if len(members) == 0 {
		return nil, errs.New("no member clusters found")
	}
	for _, member := range members {
		// also check that the member cluster name matches because the api endpoint is the same for both members
		// in the e2e tests because a single cluster is used for testing multi-member scenarios
		if member.APIEndpoint == apiEndpoint && member.Name == clusterName {
			apiURL, err := s.getMemberURL(proxyPluginName, member)
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

func (s *ServiceImpl) getMemberURL(proxyPluginName string, member *cluster.CachedToolchainCluster) (*url.URL, error) {
	if member == nil {
		return nil, errs.New("nil member provided")
	}
	if len(proxyPluginName) == 0 {
		return url.Parse(member.APIEndpoint)
	}
	if member.Client == nil {
		return nil, errs.New(fmt.Sprintf("client for member %s not set", member.Name))
	}
	proxyCfg, err := s.Services().InformerService().GetProxyPluginConfig(proxyPluginName)
	if err != nil {
		return nil, errs.New(fmt.Sprintf("unable to get proxy config %s: %s", proxyPluginName, err.Error()))
	}
	if proxyCfg.Spec.OpenShiftRouteTargetEndpoint == nil {
		return nil, errs.New(fmt.Sprintf("the proxy plugin config %s does not define an openshift route endpoint", proxyPluginName))
	}
	routeNamespace := proxyCfg.Spec.OpenShiftRouteTargetEndpoint.Namespace
	routeName := proxyCfg.Spec.OpenShiftRouteTargetEndpoint.Name

	proxyRoute := &routev1.Route{}
	key := types.NamespacedName{
		Namespace: routeNamespace,
		Name:      routeName,
	}
	err = member.Client.Get(context.Background(), key, proxyRoute)
	if err != nil {
		return nil, err
	}
	if len(proxyRoute.Status.Ingress) == 0 {
		return nil, fmt.Errorf("the route %q has not initialized to the point where the status ingress is populated", key.String())
	}

	scheme := ""
	port := proxyRoute.Spec.Port
	switch {
	case port != nil && port.TargetPort.String() == "http":
		scheme = "http://"
	case port != nil && port.TargetPort.String() == "https":
		scheme = "https://"
	default:
		scheme = "https://"
	}
	return url.Parse(scheme + proxyRoute.Status.Ingress[0].Host)

}
