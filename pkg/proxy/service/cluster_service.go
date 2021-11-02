package service

import (
	"context"
	"fmt"
	"net/url"

	"github.com/gin-gonic/gin"

	"github.com/codeready-toolchain/registration-service/pkg/log"

	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/base"
	servicecontext "github.com/codeready-toolchain/registration-service/pkg/application/service/context"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/namespace"
	"github.com/codeready-toolchain/toolchain-common/pkg/cluster"

	errs "github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// ServiceImpl represents the implementation of the signup service.
type ServiceImpl struct {
	base.BaseService
	getMembersFunc cluster.GetMemberClustersFunc
}

// NewToolchainClusterService creates a service object for performing toolchain cluster related activities.
func NewToolchainClusterService(context servicecontext.ServiceContext) service.ToolchainClusterService {
	return &ServiceImpl{
		BaseService:    base.NewBaseService(context),
		getMembersFunc: cluster.GetMemberClusters,
	}
}

func (s *ServiceImpl) GetNamespace(ctx *gin.Context, userID string) (*namespace.Namespace, error) {
	// Get Signup
	signup, err := s.ServiceContext.Services().SignupService().GetSignup(userID)
	if err != nil {
		return nil, err
	}
	if signup == nil || !signup.Status.Ready {
		return nil, errs.New("user is not (yet) provisioned")
	}

	// Get the target member
	members := s.getMembersFunc()
	if len(members) == 0 {
		return nil, errs.New("no member clusters found")
	}
	for _, member := range members {
		if member.APIEndpoint == signup.APIEndpoint {
			// Obtain the SA token
			targetNamespace := fmt.Sprintf("%s", signup.CompliantUsername)  // TODO construct the workspace namespace
			saName := fmt.Sprintf("appstudio-%s", signup.CompliantUsername) // TODO construct the sa namespace
			saNamespacedName := types.NamespacedName{Namespace: targetNamespace, Name: saName}
			sa := &v1.ServiceAccount{}
			if err := member.Client.Get(context.TODO(), saNamespacedName, sa); err != nil {
				return nil, err
			}

			for _, secret := range sa.Secrets {
				secretNamespacedName := types.NamespacedName{Namespace: targetNamespace, Name: secret.Name}
				s := &v1.Secret{}
				log.Info(ctx, fmt.Sprintf("Getting secret %v", secretNamespacedName))
				if err := member.Client.Get(context.TODO(), secretNamespacedName, s); err != nil {
					return nil, err
				}
				if s.Annotations["kubernetes.io/created-by"] == "openshift.io/create-dockercfg-secrets" {
					// There are two secrets/tokens for the SA and both are valid
					// but let's always use the non-docker one for the sake of consistency
					log.Info(nil, fmt.Sprintf("Skipping docker secret with the creted-by label: %v", secretNamespacedName))
					continue
				}
				decodedToken, ok := s.Data["token"]
				if !ok {
					log.Info(ctx, fmt.Sprintf("Skipping secret with no data.token: %v", secretNamespacedName))
					continue // It still might be the docker configuration token even if it doesn't have the "kubernetes.io/created-by" annotation
				}
				tokenStr := string(decodedToken)

				apiURL, err := url.Parse(member.APIEndpoint)
				if err != nil {
					return nil, err
				}
				return &namespace.Namespace{
					Username:           signup.CompliantUsername,
					ClusterName:        member.Name,
					ApiURL:             *apiURL,
					Namespace:          targetNamespace,
					Workspace:          targetNamespace, // TODO
					TargetClusterToken: tokenStr,
				}, nil
			}
			return nil, errs.New("no SA found for the user")
		}
	}

	return nil, errs.New("no member cluster found for the user")
}
