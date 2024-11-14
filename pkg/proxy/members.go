package proxy

import (
	"context"
	"fmt"
	"net/url"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/pkg/namespaced"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/access"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/toolchain-common/pkg/cluster"

	routev1 "github.com/openshift/api/route/v1"

	errs "github.com/pkg/errors"

	"k8s.io/apimachinery/pkg/types"
)

// MemberClusters is a type that helps with retrieving access to a specific member cluster
type MemberClusters struct { // nolint:revive
	namespaced.Client
	SignupService  service.SignupService
	GetMembersFunc cluster.GetMemberClustersFunc
}

// NewMemberClusters creates an instance of the MemberClusters type
func NewMemberClusters(client namespaced.Client, signupService service.SignupService, getMembersFunc cluster.GetMemberClustersFunc) *MemberClusters {
	si := &MemberClusters{
		Client:         client,
		SignupService:  signupService,
		GetMembersFunc: getMembersFunc,
	}
	return si
}

func (s *MemberClusters) GetClusterAccess(userID, username, workspace, proxyPluginName string, publicViewerEnabled bool) (*access.ClusterAccess, error) {
	// if workspace is not provided then return the default space access
	if workspace == "" {
		return s.getClusterAccessForDefaultWorkspace(userID, username, proxyPluginName)
	}

	return s.getSpaceAccess(userID, username, workspace, proxyPluginName, publicViewerEnabled)
}

// getSpaceAccess retrieves space access for an user
func (s *MemberClusters) getSpaceAccess(userID, username, workspace, proxyPluginName string, publicViewerEnabled bool) (*access.ClusterAccess, error) {
	// retrieve the user's complaint name
	complaintUserName, err := s.getUserSignupComplaintName(userID, username, publicViewerEnabled)
	if err != nil {
		return nil, err
	}

	// look up space
	space := &toolchainv1alpha1.Space{}
	if err := s.Get(context.TODO(), s.NamespacedName(workspace), space); err != nil {
		// log the actual error but do not return it so that it doesn't reveal information about a space that may not belong to the requestor
		log.Error(nil, err, "unable to get target cluster for workspace "+workspace)
		return nil, fmt.Errorf("the requested space is not available")
	}

	return s.accessForSpace(space, complaintUserName, proxyPluginName)
}

func (s *MemberClusters) getUserSignupComplaintName(userID, username string, publicViewerEnabled bool) (string, error) {
	// if PublicViewer is enabled and the requested user is the PublicViewer, than no lookup is required
	if publicViewerEnabled && username == toolchainv1alpha1.KubesawAuthenticatedUsername {
		return username, nil
	}

	// retrieve the UserSignup from cache
	userSignup, err := s.getSignupFromInformerForProvisionedUser(userID, username)
	if err != nil {
		return "", err
	}

	return userSignup.CompliantUsername, nil
}

// getClusterAccessForDefaultWorkspace retrieves the cluster for the user's default workspace
func (s *MemberClusters) getClusterAccessForDefaultWorkspace(userID, username, proxyPluginName string) (*access.ClusterAccess, error) {
	// retrieve the UserSignup from cache
	userSignup, err := s.getSignupFromInformerForProvisionedUser(userID, username)
	if err != nil {
		return nil, err
	}

	// retrieve user's access for cluster
	return s.accessForCluster(userSignup.APIEndpoint, userSignup.ClusterName, userSignup.CompliantUsername, proxyPluginName)
}

func (s *MemberClusters) getSignupFromInformerForProvisionedUser(userID, username string) (*signup.Signup, error) {
	// don't check for usersignup complete status, since it might cause the proxy blocking the request
	// and returning an error when quick transitions from ready to provisioning are happening.
	userSignup, err := s.SignupService.GetSignup(nil, userID, username, false)
	if err != nil {
		return nil, err
	}

	// if signup has the CompliantUsername set it means that MUR was created and useraccount is provisioned
	if userSignup == nil || userSignup.CompliantUsername == "" {
		cause := errs.New("user is not provisioned (yet)")
		log.Error(nil, cause, fmt.Sprintf("signup object: %+v", userSignup))
		return nil, cause
	}

	return userSignup, nil
}

func (s *MemberClusters) accessForSpace(space *toolchainv1alpha1.Space, username, proxyPluginName string) (*access.ClusterAccess, error) {
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

func (s *MemberClusters) accessForCluster(apiEndpoint, clusterName, username, proxyPluginName string) (*access.ClusterAccess, error) {
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

func (s *MemberClusters) getMemberURL(proxyPluginName string, member *cluster.CachedToolchainCluster) (*url.URL, error) {
	if member == nil {
		return nil, errs.New("nil member provided")
	}
	if len(proxyPluginName) == 0 {
		return url.Parse(member.APIEndpoint)
	}
	if member.Client == nil {
		return nil, errs.New(fmt.Sprintf("client for member %s not set", member.Name))
	}
	proxyCfg := &toolchainv1alpha1.ProxyPlugin{}
	if err := s.Get(context.TODO(), s.NamespacedName(proxyPluginName), proxyCfg); err != nil {
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
	err := member.Client.Get(context.Background(), key, proxyRoute)
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
