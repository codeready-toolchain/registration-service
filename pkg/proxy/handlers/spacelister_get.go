package handlers

import (
	gocontext "context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	regsercontext "github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/namespaced"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/metrics"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/toolchain-common/pkg/cluster"
	commonproxy "github.com/codeready-toolchain/toolchain-common/pkg/proxy"
	"github.com/codeready-toolchain/toolchain-common/pkg/spacebinding"
	"github.com/labstack/echo/v4"
	errs "github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func HandleSpaceGetRequest(spaceLister *SpaceLister, GetMembersFunc cluster.GetMemberClustersFunc) echo.HandlerFunc {
	// get specific workspace
	return func(ctx echo.Context) error {
		requestReceivedTime := ctx.Get(regsercontext.RequestReceivedTime).(time.Time)
		workspace, err := GetUserWorkspaceWithBindings(ctx, spaceLister, ctx.Param("workspace"), GetMembersFunc)
		if err != nil {
			spaceLister.ProxyMetrics.RegServWorkspaceHistogramVec.WithLabelValues(fmt.Sprintf("%d", http.StatusInternalServerError), metrics.MetricsLabelVerbGet).Observe(time.Since(requestReceivedTime).Seconds()) // using list as the default value for verb to minimize label combinations for prometheus to process
			return errorResponse(ctx, apierrors.NewInternalError(err))
		}
		if workspace == nil {
			// not found
			spaceLister.ProxyMetrics.RegServWorkspaceHistogramVec.WithLabelValues(fmt.Sprintf("%d", http.StatusNotFound), metrics.MetricsLabelVerbGet).Observe(time.Since(requestReceivedTime).Seconds())
			r := schema.GroupResource{Group: "toolchain.dev.openshift.com", Resource: "workspaces"}
			return errorResponse(ctx, apierrors.NewNotFound(r, ctx.Param("workspace")))
		}
		spaceLister.ProxyMetrics.RegServWorkspaceHistogramVec.WithLabelValues(fmt.Sprintf("%d", http.StatusOK), metrics.MetricsLabelVerbGet).Observe(time.Since(requestReceivedTime).Seconds())
		return getWorkspaceResponse(ctx, workspace)
	}
}

// GetUserWorkspace returns a workspace object with the required fields used by the proxy.
func GetUserWorkspace(ctx echo.Context, spaceLister *SpaceLister, workspaceName string) (*toolchainv1alpha1.Workspace, error) {
	userSignup, space, err := getUserSignupAndSpace(ctx, spaceLister, workspaceName)
	if err != nil {
		return nil, err
	}
	// signup is not ready
	if userSignup == nil || space == nil {
		return nil, nil
	}

	// retrieve user space binding
	userSpaceBinding, err := getUserOrPublicViewerSpaceBinding(ctx, spaceLister, space, userSignup, workspaceName)
	if err != nil {
		return nil, err
	}
	// consider this as not found
	if userSpaceBinding == nil {
		return nil, nil
	}

	// create and return the result workspace object
	return createWorkspaceObject(userSignup.Name, space, userSpaceBinding), nil
}

// getUserOrPublicViewerSpaceBinding retrieves the user space binding for an user and a space.
// If the SpaceBinding is not found and the PublicViewer feature is enabled, it will retry
// with the PublicViewer credentials.
func getUserOrPublicViewerSpaceBinding(ctx echo.Context, spaceLister *SpaceLister, space *toolchainv1alpha1.Space, userSignup *signup.Signup, workspaceName string) (*toolchainv1alpha1.SpaceBinding, error) {
	userSpaceBinding, err := getUserSpaceBinding(spaceLister, space, userSignup.CompliantUsername)
	if err != nil {
		ctx.Logger().Errorf("error checking if SpaceBinding is present for user %s and the workspace %s: %v", toolchainv1alpha1.KubesawAuthenticatedUsername, workspaceName, err)
		return nil, err
	}

	// if user space binding is not found and PublicViewer is enabled,
	// retry with PublicViewer's signup
	if userSpaceBinding == nil {
		if context.IsPublicViewerEnabled(ctx) {
			pvSb, err := getUserSpaceBinding(spaceLister, space, toolchainv1alpha1.KubesawAuthenticatedUsername)
			if err != nil {
				ctx.Logger().Errorf("error checking if SpaceBinding is present for user %s and the workspace %s: %v", toolchainv1alpha1.KubesawAuthenticatedUsername, workspaceName, err)
				return nil, err
			}
			if pvSb == nil {
				ctx.Logger().Errorf("unauthorized access - there is no SpaceBinding present for the user %s or %s and the workspace %s", userSignup.CompliantUsername, toolchainv1alpha1.KubesawAuthenticatedUsername, workspaceName)
				return nil, nil
			}
			return pvSb, nil
		}
		ctx.Logger().Errorf("unauthorized access - there is no SpaceBinding present for the user %s and the workspace %s", userSignup.CompliantUsername, workspaceName)
	}

	return userSpaceBinding, nil
}

// getUserSpaceBinding retrieves the space binding for an user and a space.
// If no space binding is found for the user and space then it returns nil, nil.
// If more than one space bindings are found, then it returns an error.
func getUserSpaceBinding(spaceLister *SpaceLister, space *toolchainv1alpha1.Space, compliantUsername string) (*toolchainv1alpha1.SpaceBinding, error) {
	// recursively get all the spacebindings for the current workspace
	spaceBindingLister := NewLister(spaceLister.Client, compliantUsername)
	userSpaceBindings, err := spaceBindingLister.ListForSpace(space, []toolchainv1alpha1.SpaceBinding{})
	if err != nil {
		return nil, err
	}
	if len(userSpaceBindings) == 0 {
		//  consider this as not found
		return nil, nil
	}

	if len(userSpaceBindings) > 1 {
		userBindingsErr := fmt.Errorf("invalid number of SpaceBindings found for MUR:%s and Space:%s. Expected 1 got %d", compliantUsername, space.Name, len(userSpaceBindings))
		return nil, userBindingsErr
	}

	return &userSpaceBindings[0], nil
}

func NewLister(nsClient namespaced.Client, withMurName string) *spacebinding.Lister {
	listSpaceBindingsFunc := func(spaceName string) ([]toolchainv1alpha1.SpaceBinding, error) {
		labelSelector := runtimeclient.MatchingLabels{
			toolchainv1alpha1.SpaceBindingSpaceLabelKey: spaceName,
		}
		if withMurName != "" {
			labelSelector[toolchainv1alpha1.SpaceBindingMasterUserRecordLabelKey] = withMurName
		}
		bindings := &toolchainv1alpha1.SpaceBindingList{}
		if err := nsClient.List(gocontext.TODO(), bindings, runtimeclient.InNamespace(nsClient.Namespace), labelSelector); err != nil {
			return nil, err
		}
		return bindings.Items, nil
	}
	return spacebinding.NewLister(listSpaceBindingsFunc, func(spaceName string) (*toolchainv1alpha1.Space, error) {
		space := &toolchainv1alpha1.Space{}
		return space, nsClient.Get(gocontext.TODO(), nsClient.NamespacedName(spaceName), space)
	})
}

// GetUserWorkspaceWithBindings returns a workspace object with the required fields+bindings (the list with all the users access details).
func GetUserWorkspaceWithBindings(ctx echo.Context, spaceLister *SpaceLister, workspaceName string, GetMembersFunc cluster.GetMemberClustersFunc) (*toolchainv1alpha1.Workspace, error) {
	userSignup, space, err := getUserSignupAndSpace(ctx, spaceLister, workspaceName)
	if err != nil {
		return nil, err
	}
	// signup is not ready
	if userSignup == nil || space == nil {
		return nil, nil
	}

	// recursively get all the spacebindings for the current workspace
	spaceBindingLister := NewLister(spaceLister.Client, "")
	allSpaceBindings, err := spaceBindingLister.ListForSpace(space, []toolchainv1alpha1.SpaceBinding{})
	if err != nil {
		ctx.Logger().Error(err, "failed to list space bindings")
		return nil, err
	}

	// check if user has access to this workspace
	userBinding := filterUserSpaceBinding(userSignup.CompliantUsername, allSpaceBindings)
	if userBinding == nil {
		// if PublicViewer is enabled, check if the Space is visibile to PublicViewer
		// in case usersignup is the KubesawAuthenticatedUsername, then we already checked in the previous step
		if context.IsPublicViewerEnabled(ctx) && userSignup.CompliantUsername != toolchainv1alpha1.KubesawAuthenticatedUsername {
			userBinding = filterUserSpaceBinding(toolchainv1alpha1.KubesawAuthenticatedUsername, allSpaceBindings)
		}

		if userBinding == nil {
			//  let's only log the issue and consider this as not found
			ctx.Logger().Errorf("unauthorized access - there is no SpaceBinding present for the user %s and the workspace %s", userSignup.CompliantUsername, workspaceName)
			return nil, nil
		}
	}

	// list all SpaceBindingRequests , just in case there might be some failing to create a SpaceBinding resource.
	allSpaceBindingRequests, err := listSpaceBindingRequestsForSpace(ctx, GetMembersFunc, space)
	if err != nil {
		return nil, err
	}

	// build the Bindings list with the available actions
	// this field is populated only for the GET workspace request
	bindings, err := generateWorkspaceBindings(space, allSpaceBindings, allSpaceBindingRequests)
	if err != nil {
		ctx.Logger().Error(errs.Wrap(err, "unable to generate bindings field"))
		return nil, err
	}

	// add available roles, this field is populated only for the GET workspace request
	nsTemplateTier := &toolchainv1alpha1.NSTemplateTier{}
	if err := spaceLister.Get(ctx.Request().Context(), spaceLister.NamespacedName(space.Spec.TierName), nsTemplateTier); err != nil {
		ctx.Logger().Error(errs.Wrap(err, "unable to get nstemplatetier"))
		return nil, err
	}

	return createWorkspaceObject(userSignup.Name, space, userBinding,
		commonproxy.WithAvailableRoles(getRolesFromNSTemplateTier(nsTemplateTier)),
		commonproxy.WithBindings(bindings),
	), nil
}

// getUserSignupAndSpace returns the space and the usersignup for a given request.
// When no space is found a nil value is returned instead of an error.
func getUserSignupAndSpace(ctx echo.Context, spaceLister *SpaceLister, workspaceName string) (*signup.Signup, *toolchainv1alpha1.Space, error) {
	userSignup, err := spaceLister.GetProvisionedUserSignup(ctx)
	if err != nil {
		return nil, nil, err
	}
	if userSignup == nil && context.IsPublicViewerEnabled(ctx) {
		userSignup = &signup.Signup{
			CompliantUsername: toolchainv1alpha1.KubesawAuthenticatedUsername,
			Name:              toolchainv1alpha1.KubesawAuthenticatedUsername,
		}
	}
	space := &toolchainv1alpha1.Space{}
	if err := spaceLister.Get(gocontext.TODO(), spaceLister.NamespacedName(workspaceName), space); err != nil {
		ctx.Logger().Error(errs.Wrap(err, "unable to get space"))
		return userSignup, nil, nil
	}
	return userSignup, space, err
}

// listSpaceBindingRequestsForSpace searches for the SpaceBindingRequests in all the provisioned namespaces for the given Space
func listSpaceBindingRequestsForSpace(ctx echo.Context, GetMembersFunc cluster.GetMemberClustersFunc, space *toolchainv1alpha1.Space) ([]toolchainv1alpha1.SpaceBindingRequest, error) {
	members := GetMembersFunc()
	if len(members) == 0 {
		return nil, errs.New("no member clusters found")
	}
	var allSbrs []toolchainv1alpha1.SpaceBindingRequest
	for _, member := range members {
		if member.Name == space.Status.TargetCluster {
			// list sbrs in all namespaces for the current space
			for _, namespace := range space.Status.ProvisionedNamespaces {
				sbrs := &toolchainv1alpha1.SpaceBindingRequestList{}
				if err := member.Client.List(ctx.Request().Context(), sbrs, runtimeclient.InNamespace(namespace.Name)); err != nil {
					return nil, err
				}
				allSbrs = append(allSbrs, sbrs.Items...)
			}
		}
	}
	return allSbrs, nil
}

func getRolesFromNSTemplateTier(nstemplatetier *toolchainv1alpha1.NSTemplateTier) []string {
	roles := make([]string, 0, len(nstemplatetier.Spec.SpaceRoles))
	for k := range nstemplatetier.Spec.SpaceRoles {
		roles = append(roles, k)
	}
	sort.Strings(roles)
	return roles
}

// filterUserSpaceBinding returns the spacebinding for a given username, or nil if not found
func filterUserSpaceBinding(username string, allSpaceBindings []toolchainv1alpha1.SpaceBinding) *toolchainv1alpha1.SpaceBinding {
	for _, binding := range allSpaceBindings {
		if binding.Spec.MasterUserRecord == username {
			return &binding
		}
	}
	return nil
}

// generateWorkspaceBindings generates the bindings list starting from the spacebindings found on a given space resource and an all parent spaces.
// The Bindings list  has the available actions for each entry in the list.
func generateWorkspaceBindings(space *toolchainv1alpha1.Space, spaceBindings []toolchainv1alpha1.SpaceBinding, spacebindingRequests []toolchainv1alpha1.SpaceBindingRequest) ([]toolchainv1alpha1.Binding, error) {
	bindings := make([]toolchainv1alpha1.Binding, 0, len(spaceBindings)+len(spacebindingRequests))
	for _, spaceBinding := range spaceBindings {
		binding := toolchainv1alpha1.Binding{
			MasterUserRecord: spaceBinding.Spec.MasterUserRecord,
			Role:             spaceBinding.Spec.SpaceRole,
			AvailableActions: []string{},
		}
		spaceBindingSpaceName := spaceBinding.Labels[toolchainv1alpha1.SpaceBindingSpaceLabelKey]
		sbrName, sbrNameFound := spaceBinding.Labels[toolchainv1alpha1.SpaceBindingRequestLabelKey]
		sbrNamespace, sbrNamespaceFound := spaceBinding.Labels[toolchainv1alpha1.SpaceBindingRequestNamespaceLabelKey]
		// check if spacebinding was generated from SBR on the current space and not on a parentSpace.
		if (sbrNameFound || sbrNamespaceFound) && spaceBindingSpaceName == space.GetName() {
			if sbrName == "" {
				// small corner case where the SBR name for some reason is not present as labels on the sb.
				return nil, fmt.Errorf("SpaceBindingRequest name not found on binding: %s", spaceBinding.GetName())
			}
			if sbrNamespace == "" {
				// small corner case where the SBR namespace for some reason is not present as labels on the sb.
				return nil, fmt.Errorf("SpaceBindingRequest namespace not found on binding: %s", spaceBinding.GetName())
			}
			// this is a binding that was created via SpaceBindingRequest, it can be updated or deleted
			binding.AvailableActions = []string{UpdateBindingAction, DeleteBindingAction}
			binding.BindingRequest = &toolchainv1alpha1.BindingRequest{
				Name:      sbrName,
				Namespace: sbrNamespace,
			}
		} else if spaceBindingSpaceName != space.GetName() {
			// this is a binding that was inherited from a parent space, since the name on the spacebinding label doesn't match with the current space name.
			// It can only be overridden by another SpaceBindingRequest containing the same MUR but different role.
			binding.AvailableActions = []string{OverrideBindingAction}
		} else {
			// this is a system generated SpaceBinding, so it cannot be managed by workspace users.
			binding.AvailableActions = []string{}
		}
		bindings = append(bindings, binding)
	}

	// add also all the spacebinding requests
	for i := range spacebindingRequests {
		// add to binding list only if it was not added already by the SpaceBinding search above
		sbr := &spacebindingRequests[i]
		if alreadyInBindingList(bindings, sbr) {
			continue
		}

		binding := toolchainv1alpha1.Binding{
			MasterUserRecord: sbr.Spec.MasterUserRecord,
			Role:             sbr.Spec.SpaceRole,
			AvailableActions: []string{UpdateBindingAction, DeleteBindingAction},
		}
		binding.BindingRequest = &toolchainv1alpha1.BindingRequest{
			Name:      sbr.Name,
			Namespace: sbr.Namespace,
		}
		bindings = append(bindings, binding)
	}

	// let's sort the list based on username,
	// in order to make it deterministic
	sort.Slice(bindings, func(i, j int) bool {
		return bindings[i].MasterUserRecord < bindings[j].MasterUserRecord
	})

	return bindings, nil
}

// alreadyInBindingList searches the binding list for the given SpaceBindingRequest
// returns true if the SBR is already present
// returns false if the SBR was not found
func alreadyInBindingList(bindings []toolchainv1alpha1.Binding, sbr *toolchainv1alpha1.SpaceBindingRequest) bool {
	for _, existingBinding := range bindings {
		if existingBinding.BindingRequest != nil && existingBinding.BindingRequest.Name == sbr.Name && existingBinding.BindingRequest.Namespace == sbr.Namespace {
			return true
		}
	}
	return false
}

func getWorkspaceResponse(ctx echo.Context, workspace *toolchainv1alpha1.Workspace) error {
	ctx.Response().Writer.Header().Set("Content-Type", "application/json")
	ctx.Response().Writer.WriteHeader(http.StatusOK)
	return json.NewEncoder(ctx.Response().Writer).Encode(workspace)
}
