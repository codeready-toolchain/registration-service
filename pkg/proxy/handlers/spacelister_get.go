package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/metrics"
	commonproxy "github.com/codeready-toolchain/toolchain-common/pkg/proxy"
	"github.com/codeready-toolchain/toolchain-common/pkg/spacebinding"
	"github.com/labstack/echo/v4"
	errs "github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
)

func (s *SpaceLister) HandleSpaceGetRequest(ctx echo.Context) error {
	// get specific workspace
	requestReceivedTime := ctx.Get(context.RequestReceivedTime).(time.Time)
	workspace, err := s.GetUserWorkspace(ctx, ctx.Param("workspace"))
	if err != nil {
		s.ProxyMetrics.RegServWorkspaceHistogramVec.WithLabelValues(fmt.Sprintf("%d", http.StatusInternalServerError), metrics.MetricsLabelVerbGet).Observe(time.Since(requestReceivedTime).Seconds()) // using list as the default value for verb to minimize label combinations for prometheus to process
		return errorResponse(ctx, apierrors.NewInternalError(err))
	}
	if workspace == nil {
		// not found
		s.ProxyMetrics.RegServWorkspaceHistogramVec.WithLabelValues(fmt.Sprintf("%d", http.StatusNotFound), metrics.MetricsLabelVerbGet).Observe(time.Since(requestReceivedTime).Seconds())
		r := schema.GroupResource{Group: "toolchain.dev.openshift.com", Resource: "workspaces"}
		return errorResponse(ctx, apierrors.NewNotFound(r, ctx.Param("workspace")))
	}
	s.ProxyMetrics.RegServWorkspaceHistogramVec.WithLabelValues(fmt.Sprintf("%d", http.StatusOK), metrics.MetricsLabelVerbGet).Observe(time.Since(requestReceivedTime).Seconds())
	return getWorkspaceResponse(ctx, workspace)
}

func (s *SpaceLister) GetUserWorkspace(ctx echo.Context, workspaceName string) (*toolchainv1alpha1.Workspace, error) {
	userSignup, err := s.GetProvisionedUserSignup(ctx)
	if err != nil {
		return nil, err
	}
	// signup is not ready
	if userSignup == nil {
		return nil, nil
	}
	space, err := s.GetInformerServiceFunc().GetSpace(workspaceName)
	if err != nil {
		ctx.Logger().Error(errs.Wrap(err, "unable to get space"))
		return nil, nil
	}

	// recursively get all the spacebindings for the current workspace
	listSpaceBindingsFunc := func(spaceName string) ([]toolchainv1alpha1.SpaceBinding, error) {
		spaceSelector, err := labels.NewRequirement(toolchainv1alpha1.SpaceBindingSpaceLabelKey, selection.Equals, []string{spaceName})
		if err != nil {
			return nil, err
		}
		return s.GetInformerServiceFunc().ListSpaceBindings(*spaceSelector)
	}
	spaceBindingLister := spacebinding.NewLister(listSpaceBindingsFunc, s.GetInformerServiceFunc().GetSpace)
	allSpaceBindings, err := spaceBindingLister.ListForSpace(space, []toolchainv1alpha1.SpaceBinding{})
	if err != nil {
		ctx.Logger().Error(err, "failed to list space bindings")
		return nil, err
	}

	// check if user has access to this workspace
	userBinding := filterUserSpaceBinding(userSignup.CompliantUsername, allSpaceBindings)
	if userBinding == nil {
		//  let's only log the issue and consider this as not found
		ctx.Logger().Error(fmt.Sprintf("unauthorized access - there is no SpaceBinding present for the user %s and the workspace %s", userSignup.CompliantUsername, workspaceName))
		return nil, nil
	}
	// build the Bindings list with the available actions
	// this field is populated only for the GET workspace request
	bindings, err := generateWorkspaceBindings(space, allSpaceBindings)
	if err != nil {
		ctx.Logger().Error(errs.Wrap(err, "unable to generate bindings field"))
		return nil, err
	}

	// add available roles, this field is populated only for the GET workspace request
	nsTemplateTier, err := s.GetInformerServiceFunc().GetNSTemplateTier(space.Spec.TierName)
	if err != nil {
		ctx.Logger().Error(errs.Wrap(err, "unable to get nstemplatetier"))
		return nil, err
	}

	return createWorkspaceObject(userSignup.Name, space, userBinding,
		commonproxy.WithAvailableRoles(getRolesFromNSTemplateTier(nsTemplateTier)),
		commonproxy.WithBindings(bindings),
	), nil
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
func generateWorkspaceBindings(space *toolchainv1alpha1.Space, spaceBindings []toolchainv1alpha1.SpaceBinding) ([]toolchainv1alpha1.Binding, error) {
	var bindings []toolchainv1alpha1.Binding
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

	// let's sort the list based on username,
	// in order to make it deterministic
	sort.Slice(bindings, func(i, j int) bool {
		return bindings[i].MasterUserRecord < bindings[j].MasterUserRecord
	})

	return bindings, nil
}

func getWorkspaceResponse(ctx echo.Context, workspace *toolchainv1alpha1.Workspace) error {
	ctx.Response().Writer.Header().Set("Content-Type", "application/json")
	ctx.Response().Writer.WriteHeader(http.StatusOK)
	return json.NewEncoder(ctx.Response().Writer).Encode(workspace)
}
