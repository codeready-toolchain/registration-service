package service

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/base"
	servicecontext "github.com/codeready-toolchain/registration-service/pkg/application/service/context"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/namespace"
	"github.com/codeready-toolchain/toolchain-common/pkg/cluster"

	"github.com/gin-gonic/gin"
	errs "github.com/pkg/errors"
	authv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
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

func (s *ServiceImpl) GetNamespace(ctx *gin.Context, userID, username string) (*namespace.NamespaceAccess, error) {
	// Get Signup
	signup, err := s.ServiceContext.Services().SignupService().GetSignup(userID, username)
	if err != nil {
		return nil, err
	}
	if signup == nil || !signup.Status.Ready {
		return nil, errs.New("user is not (yet) provisioned")
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
			// Obtain the SA token
			targetNamespace := signup.CompliantUsername
			saName := fmt.Sprintf("appstudio-%s", signup.CompliantUsername)
			saNamespacedName := types.NamespacedName{Namespace: targetNamespace, Name: saName}
			cfg := member.Config.RestConfig
			if cfg.ContentConfig.GroupVersion == nil {
				cfg.ContentConfig.GroupVersion = &authv1.SchemeGroupVersion
			}
			if cfg.ContentConfig.NegotiatedSerializer == nil {
				cfg.ContentConfig.NegotiatedSerializer = scheme.Codecs
			}
			cl, err := rest.RESTClientFor(cfg)
			if err != nil {
				return nil, err
			}
			tokenStr, err := getServiceAccountToken(cl, saNamespacedName)
			if err != nil {
				return nil, err
			}
			apiURL, err := url.Parse(member.APIEndpoint)
			if err != nil {
				return nil, err
			}
			return namespace.NewNamespaceAccess(*apiURL, tokenStr, member.Client), nil
		}
	}

	return nil, errs.New("no member cluster found for the user")
}

// getServiceAccountToken returns the SA's token or returns an error if none was found.
// NOTE: due to a changes in OpenShift 4.11, tokens are not listed as `secrets` in ServiceAccounts.
// The recommended solution is to use the TokenRequest API when server version >= 4.11
// (see https://docs.openshift.com/container-platform/4.11/release_notes/ocp-4-11-release-notes.html#ocp-4-11-notable-technical-changes)
func getServiceAccountToken(cl *rest.RESTClient, namespacedName types.NamespacedName) (string, error) {
	tokenRequest := &authv1.TokenRequest{
		Spec: authv1.TokenRequestSpec{
			ExpirationSeconds: pointer.Int64(int64(365 * 24 * time.Hour / time.Second)), // token will be valid for 1 year
		},
	}
	result := &authv1.TokenRequest{}
	if err := cl.Post().
		AbsPath(fmt.Sprintf("api/v1/namespaces/%s/serviceaccounts/%s/token", namespacedName.Namespace, namespacedName.Name)).
		Body(tokenRequest).
		Do(context.TODO()).
		Into(result); err != nil {
		return "", err
	}
	return result.Status.Token, nil
}
