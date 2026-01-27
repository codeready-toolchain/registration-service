package namespaces

import (
	"context"
	"fmt"
	"strings"
	"time"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	customCtx "github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/namespaced"
	"github.com/codeready-toolchain/toolchain-common/pkg/cluster"
	"github.com/gin-gonic/gin"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// requestTimeout defines the maximum time the requests of the namespace
// manager can take before timing out.
const requestTimeout = 5 * time.Second

// ErrUserSignUpNotFoundDeactivated is a custom error type used for signaling
// that the user signup was not found or deactivated. Useful for error type
// checks in the handler or any other code that calls the manager.
type ErrUserSignUpNotFoundDeactivated struct{}

func (ErrUserSignUpNotFoundDeactivated) Error() string {
	return "the specified user was not found or is deactivated"
}

// ErrUserHasNoProvisionedNamespaces is a custom error type used for signaling
// that the user has no provisioned namespaces in the cluster. Useful for
// error type checks in the handler or any other code that calls the manager.
type ErrUserHasNoProvisionedNamespaces struct {
	memberClusterName string
	nsTemplateSetName string
}

func NewErrUserHasNoProvisionedNamespaces(memberClusterName string, nsTemplateSetName string) ErrUserHasNoProvisionedNamespaces {
	return ErrUserHasNoProvisionedNamespaces{
		memberClusterName: memberClusterName,
		nsTemplateSetName: nsTemplateSetName,
	}
}

func (e ErrUserHasNoProvisionedNamespaces) Error() string {
	return fmt.Sprintf(`the associated NSTemplateSet "%s" in the member cluster "%s" does not have any provisioned namespaces`, e.nsTemplateSetName, e.memberClusterName)
}

// Manager manages the user's namespaces.
type Manager interface {
	// ResetNamespaces locates the user's namespaces in their corresponding member clusters and deletes them, so that
	// the NSTemplate controller can recreate them.
	ResetNamespaces(ginCtx *gin.Context) error
}

type manager struct {
	getMemberClustersFunc cluster.GetMemberClustersFunc
	hostNamespaceClient   namespaced.Client
	signupService         service.SignupService
}

// NewNamespacesManager creates a new instance of the manager which can be used to manage user's namespaces.
func NewNamespacesManager(getMemberClustersFunc cluster.GetMemberClustersFunc, hostNamespaceClient namespaced.Client, signupService service.SignupService) Manager {
	return &manager{
		getMemberClustersFunc: getMemberClustersFunc,
		hostNamespaceClient:   hostNamespaceClient,
		signupService:         signupService,
	}
}

func (mgr *manager) ResetNamespaces(ginCtx *gin.Context) error {
	// Grab the corresponding user signup resource to get the user's compliant
	// username, since that is the one that is used across the Developer
	// Sandbox resources.
	userSignup, err := mgr.signupService.GetSignup(ginCtx, ginCtx.GetString(customCtx.UsernameKey), true)
	if err != nil {
		return fmt.Errorf("unable to obtain the user signup: %w", err)
	}

	// The SignupService might return a "nil" user signup if the user is not
	// found or is deactivated. The service can also return an empty compliant
	// username if the user is on "pending approval" state or the signup, for
	// some reason, was incomplete.
	if userSignup == nil || strings.TrimSpace(userSignup.CompliantUsername) == "" {
		return ErrUserSignUpNotFoundDeactivated{}
	}

	compliantUsername := userSignup.CompliantUsername

	// Fetch the user's space.
	userSpaceCtx, userSpaceCancel := context.WithTimeout(ginCtx.Request.Context(), requestTimeout)
	defer userSpaceCancel()

	var userSpace toolchainv1alpha1.Space
	err = mgr.hostNamespaceClient.Get(userSpaceCtx, types.NamespacedName{Namespace: mgr.hostNamespaceClient.Namespace, Name: compliantUsername}, &userSpace)
	if err != nil {
		return fmt.Errorf(`unable to get user's space resource: %w`, err)
	}

	// Get the client for the cluster in which the user's NSTemplateSet is located.
	memberClusters := mgr.getMemberClustersFunc(func(clstr *cluster.CachedToolchainCluster) bool {
		return clstr.Name == userSpace.Spec.TargetCluster
	})

	if len(memberClusters) == 0 {
		return fmt.Errorf(`unable to locate the target cluster "%s" for the user`, userSpace.Spec.TargetCluster)
	}

	// Loop through the member clusters to get the NSTemplateSet of the user and determine which namespaces need
	// to be reset.
	for _, memberCluster := range memberClusters {
		if memberCluster.Client == nil {
			return fmt.Errorf(`unable to obtain the client for cluster "%s"`, memberCluster.Name)
		}

		// Obtain the user's NSTemplateSet to be able to determine which namespaces we are deleting.
		nsTemplateSet, err := mgr.getNSTemplateSetForUser(ginCtx.Request.Context(), memberCluster, compliantUsername)
		if err != nil {
			return fmt.Errorf(`unable to get the "NSTemplateSet" resource for the user in cluster "%s": %w`, memberCluster.Name, err)
		}

		if len(nsTemplateSet.Status.ProvisionedNamespaces) == 0 {
			return NewErrUserHasNoProvisionedNamespaces(memberCluster.Name, nsTemplateSet.Name)
		}

		// Delete the given namespaces from the cluster. We use individual
		// requests instead of a single "DeleteAllOf" call because even if the
		// service account has the required permissions, the requests end up
		// failing with a "the server does not allow this method on the
		// requested resource" error.
		for _, namespace := range nsTemplateSet.Status.ProvisionedNamespaces {
			err := mgr.deleteNamespace(ginCtx.Request.Context(), memberCluster, namespace)
			if err != nil && !apierrors.IsNotFound(err) {
				return fmt.Errorf(`unable to delete user namespace "%s" in cluster "%s": %w`, namespace.Name, memberCluster.Name, err)
			}
		}
	}

	return nil
}

// getNSTemplateSetForUser gets the NSTemplateSet object of the user from the given cluster.
func (mgr *manager) getNSTemplateSetForUser(parentCtx context.Context, memberCluster *cluster.CachedToolchainCluster, compliantUsername string) (*toolchainv1alpha1.NSTemplateSet, error) {
	ctx, cancel := context.WithTimeout(parentCtx, requestTimeout)
	defer cancel()

	var nsTemplateSet toolchainv1alpha1.NSTemplateSet
	err := memberCluster.Client.Get(ctx, types.NamespacedName{Namespace: memberCluster.OperatorNamespace, Name: compliantUsername}, &nsTemplateSet)
	if err != nil {
		return nil, err
	}

	return &nsTemplateSet, nil
}

// deleteNamespace deletes the given namespace from the given cluster.
func (mgr *manager) deleteNamespace(parentCtx context.Context, memberCluster *cluster.CachedToolchainCluster, namespace toolchainv1alpha1.SpaceNamespace) error {
	ctx, cancel := context.WithTimeout(parentCtx, requestTimeout)
	defer cancel()

	return memberCluster.Client.Delete(ctx, &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace.Name}})
}
