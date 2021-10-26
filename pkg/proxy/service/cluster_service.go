package service

import (
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/proxy/namespace"

	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/base"
	servicecontext "github.com/codeready-toolchain/registration-service/pkg/application/service/context"
	common "github.com/codeready-toolchain/toolchain-common/pkg/cluster"

	//userv1 "github.com/openshift/api/user/v1"
	errs "github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubectl/pkg/scheme"
)

// ServiceImpl represents the implementation of the signup service.
type ServiceImpl struct {
	base.BaseService
	getMembersFunc common.GetMemberClustersFunc
}

// NewToolchainClusterService creates a service object for performing toolchain cluster related activities.
func NewToolchainClusterService(context servicecontext.ServiceContext) service.ToolchainClusterService {
	return &ServiceImpl{
		BaseService:    base.NewBaseService(context),
		getMembersFunc: common.GetMemberClusters,
	}
}

func (s *ServiceImpl) GetNamespace(userID string) (*namespace.Namespace, error) {
	return nil, nil
}

//// Get returns the corresponding member cluster where the user with the given OpenShift token is provisioned to.
//// Returns nil, nil if no cluster found for that token. Can happen if the token is expired.
//func (s *ServiceImpl) Get(token string) (*cluster.TokenCluster, error) {
//	// Call the whoami endpoint in each member cluster using the provided token.
//	// The cluster which will return the user instead of 401 is the cluster the user is provisioned to.
//	members := s.getMembersFunc()
//	if len(members) == 0 {
//		return nil, errors.New("no member clusters found")
//	}
//	for _, member := range members {
//		cl, err := newClusterClient(token, member.APIEndpoint)
//		if err != nil {
//			return nil, err
//		}
//
//		user := &userv1.User{}
//		if err := cl.Get().AbsPath("/apis/user.openshift.io/v1/users/~").Do(context.TODO()).Into(user); err != nil {
//			if !apierrors.IsUnauthorized(err) { // If Unauthorized then this is not the user's cluster. Need to check the next cluster.
//				return nil, err
//			}
//			// TODO check if the token is actually expired. If so then return a proper error message to the client.
//		} else {
//			return &cluster.TokenCluster{
//				Username:    user.Name,
//				ClusterName: member.Name,
//				ApiURL:      member.APIEndpoint,
//				Created:     time.Now(),
//			}, nil
//		}
//	}
//	return nil, nil
//}

func newClusterClient(token, apiEndpoint string) (*rest.RESTClient, error) {
	if err := addToScheme(); err != nil {
		return nil, err
	}
	clusterConfig, err := clientcmd.BuildConfigFromFlags(apiEndpoint, "")
	if err != nil {
		return nil, err
	}

	clusterConfig.BearerToken = string(token)
	clusterConfig.QPS = 40.0
	clusterConfig.Burst = 50
	clusterConfig.Timeout = 5 * time.Second

	cl, err := rest.RESTClientFor(clusterConfig)
	if err != nil {
		return nil, errs.Wrap(err, "cannot create cluster client")
	}
	return cl, nil
}

func addToScheme() error {
	// TODO
	var addToSchemes runtime.SchemeBuilder
	addToSchemes = append(
		addToSchemes,
	//userv1.Install
	)
	return addToSchemes.AddToScheme(scheme.Scheme)
}
