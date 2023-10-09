package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"time"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/application"
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/pkg/metrics"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	commonproxy "github.com/codeready-toolchain/toolchain-common/pkg/proxy"
	"github.com/codeready-toolchain/toolchain-common/pkg/spacebinding"
	"github.com/gin-gonic/gin"

	"github.com/labstack/echo/v4"
	errs "github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
)

const (
	// UpdateBindingAction specifies that the current binding can be updated by providing a different Space Role.
	UpdateBindingAction = "update"
	// DeleteBindingAction specifies that the current binding can be deleted in order to revoke user access to the Space.
	DeleteBindingAction = "delete"
	// OverrideBindingAction specifies that the current binding can be overridden by creating a SpaceBindingRequest containing the same MUR but different Space Role.
	OverrideBindingAction = "override"
)

type SpaceLister struct {
	GetSignupFunc          func(ctx *gin.Context, userID, username string, checkUserSignupCompleted bool) (*signup.Signup, error)
	GetInformerServiceFunc func() service.InformerService
}

func NewSpaceLister(app application.Application) *SpaceLister {
	return &SpaceLister{
		GetSignupFunc:          app.SignupService().GetSignupFromInformer,
		GetInformerServiceFunc: app.InformerService,
	}
}

func (s *SpaceLister) HandleSpaceListRequest(ctx echo.Context) error {
	// list all user workspaces
	requestReceivedTime := ctx.Get(context.RequestReceivedTime).(time.Time)
	workspaces, err := s.ListUserWorkspaces(ctx)
	if err != nil {
		metrics.RegServWorkspaceHistogramVec.WithLabelValues(fmt.Sprintf("%d", http.StatusInternalServerError), metrics.MetricsLabelVerbList).Observe(time.Since(requestReceivedTime).Seconds()) // using list as the default value for verb to minimize label combinations for prometheus to process
		return errorResponse(ctx, apierrors.NewInternalError(err))
	}
	metrics.RegServWorkspaceHistogramVec.WithLabelValues(fmt.Sprintf("%d", http.StatusOK), metrics.MetricsLabelVerbList).Observe(time.Since(requestReceivedTime).Seconds())
	return listWorkspaceResponse(ctx, workspaces)
}

func (s *SpaceLister) HandleSpaceGetRequest(ctx echo.Context) error {
	// get specific workspace
	requestReceivedTime := ctx.Get(context.RequestReceivedTime).(time.Time)
	workspace, err := s.GetUserWorkspace(ctx)
	if err != nil {
		metrics.RegServWorkspaceHistogramVec.WithLabelValues(fmt.Sprintf("%d", http.StatusInternalServerError), metrics.MetricsLabelVerbGet).Observe(time.Since(requestReceivedTime).Seconds()) // using list as the default value for verb to minimize label combinations for prometheus to process
		return errorResponse(ctx, apierrors.NewInternalError(err))
	}
	if workspace == nil {
		// not found
		metrics.RegServWorkspaceHistogramVec.WithLabelValues(fmt.Sprintf("%d", http.StatusNotFound), metrics.MetricsLabelVerbGet).Observe(time.Since(requestReceivedTime).Seconds())
		r := schema.GroupResource{Group: "toolchain.dev.openshift.com", Resource: "workspaces"}
		return errorResponse(ctx, apierrors.NewNotFound(r, ctx.Param("workspace")))
	}
	metrics.RegServWorkspaceHistogramVec.WithLabelValues(fmt.Sprintf("%d", http.StatusOK), metrics.MetricsLabelVerbGet).Observe(time.Since(requestReceivedTime).Seconds())
	return getWorkspaceResponse(ctx, workspace)
}

func (s *SpaceLister) GetUserWorkspace(ctx echo.Context) (*toolchainv1alpha1.Workspace, error) {
	userID, _ := ctx.Get(context.SubKey).(string)
	username, _ := ctx.Get(context.UsernameKey).(string)
	workspaceName := ctx.Param("workspace")

	userSignup, err := s.GetSignupFunc(nil, userID, username, false)
	if err != nil {
		cause := errs.Wrap(err, "error retrieving userSignup")
		ctx.Logger().Error(cause)
		return nil, cause
	}
	if userSignup == nil || userSignup.CompliantUsername == "" {
		// account exists but the compliant username is not set yet, meaning it has not been fully provisioned yet
		// return not found response when specific workspace request was issued
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
	getSpaceFunc := func(spaceName string) (*toolchainv1alpha1.Space, error) {
		return s.GetInformerServiceFunc().GetSpace(spaceName)
	}
	spaceBindingLister := spacebinding.NewLister(listSpaceBindingsFunc, getSpaceFunc)
	allSpaceBindings, err := spaceBindingLister.ListForSpace(space, []toolchainv1alpha1.SpaceBinding{})
	if err != nil {
		ctx.Logger().Error(err, "failed to list space bindings")
		return nil, err
	}

	// check if user has access to this workspace
	userBindings := filterUserSpaceBindings(userSignup.CompliantUsername, allSpaceBindings)
	if len(userBindings) == 0 {
		//  let's only log the issue and consider this as not found
		ctx.Logger().Error(fmt.Sprintf("expected only 1 spacebinding, got 0 for user %s and workspace %s", userSignup.CompliantUsername, workspaceName))
		return nil, nil
	} else if len(userBindings) > 1 {
		// internal server error
		cause := fmt.Errorf("expected only 1 spacebinding, got %d for user %s and workspace %s", len(userBindings), userSignup.CompliantUsername, workspaceName)
		ctx.Logger().Error(cause.Error())
		return nil, cause
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

	return createWorkspaceObject(userSignup.Name, space, &userBindings[0],
		commonproxy.WithAvailableRoles(getRolesFromNSTemplateTier(nsTemplateTier)),
		commonproxy.WithBindings(bindings),
	), nil
}

func (s *SpaceLister) ListUserWorkspaces(ctx echo.Context) ([]toolchainv1alpha1.Workspace, error) {
	userID, _ := ctx.Get(context.SubKey).(string)
	username, _ := ctx.Get(context.UsernameKey).(string)

	signup, err := s.GetSignupFunc(nil, userID, username, false)
	if err != nil {
		ctx.Logger().Error(errs.Wrap(err, "error retrieving signup"))
		return nil, err
	}
	if signup == nil || signup.CompliantUsername == "" {
		// account exists but the compliant username is not set yet, meaning it has not been fully provisioned yet, so return an empty list
		return []toolchainv1alpha1.Workspace{}, nil
	}

	murName := signup.CompliantUsername
	spaceBindings, err := s.listSpaceBindingsForUser(murName)
	if err != nil {
		ctx.Logger().Error(errs.Wrap(err, "error listing space bindings"))
		return nil, err
	}

	return s.workspacesFromSpaceBindings(signup.Name, spaceBindings), nil
}

func (s *SpaceLister) listSpaceBindingsForUser(murName string) ([]toolchainv1alpha1.SpaceBinding, error) {
	murSelector, err := labels.NewRequirement(toolchainv1alpha1.SpaceBindingMasterUserRecordLabelKey, selection.Equals, []string{murName})
	if err != nil {
		return nil, err
	}
	requirements := []labels.Requirement{*murSelector}
	return s.GetInformerServiceFunc().ListSpaceBindings(requirements...)
}

func (s *SpaceLister) workspacesFromSpaceBindings(signupName string, spaceBindings []toolchainv1alpha1.SpaceBinding) []toolchainv1alpha1.Workspace {
	workspaces := []toolchainv1alpha1.Workspace{}
	for i := range spaceBindings {
		spacebinding := &spaceBindings[i]
		space, err := s.getSpace(spacebinding)
		if err != nil {
			// log error and continue so that the api behaves in a best effort manner
			// ie. if a space isn't listed something went wrong but we still want to return the other spaces if possible
			log.Errorf(nil, err, "unable to get space", "space", spacebinding.Labels[toolchainv1alpha1.SpaceBindingSpaceLabelKey])
			continue
		}
		workspace := createWorkspaceObject(signupName, space, spacebinding)
		workspaces = append(workspaces, *workspace)
	}
	return workspaces
}

func createWorkspaceObject(signupName string, space *toolchainv1alpha1.Space, spaceBinding *toolchainv1alpha1.SpaceBinding, wsAdditionalOptions ...commonproxy.WorkspaceOption) *toolchainv1alpha1.Workspace {
	// TODO right now we get SpaceCreatorLabelKey but should get owner from Space once it's implemented
	ownerName := space.Labels[toolchainv1alpha1.SpaceCreatorLabelKey]

	wsOptions := []commonproxy.WorkspaceOption{
		commonproxy.WithNamespaces(space.Status.ProvisionedNamespaces),
		commonproxy.WithOwner(ownerName),
		commonproxy.WithRole(spaceBinding.Spec.SpaceRole),
		commonproxy.WithObjectMetaFrom(space.ObjectMeta),
	}
	// set the workspace type to "home" to indicate it is the user's home space
	// TODO set home type based on UserSignup.Status.HomeSpace once it's implemented
	if ownerName == signupName {
		wsOptions = append(wsOptions, commonproxy.WithType("home"))
	}
	wsOptions = append(wsOptions, wsAdditionalOptions...)

	workspace := commonproxy.NewWorkspace(space.GetName(), wsOptions...)
	return workspace
}

func (s *SpaceLister) getSpace(spaceBinding *toolchainv1alpha1.SpaceBinding) (*toolchainv1alpha1.Space, error) {
	spaceName := spaceBinding.Labels[toolchainv1alpha1.SpaceBindingSpaceLabelKey]
	if spaceName == "" { // space may not be initialized
		// log error and continue so that the api behaves in a best effort manner
		return nil, fmt.Errorf("spacebinding has no '%s' label", toolchainv1alpha1.SpaceBindingSpaceLabelKey)
	}
	return s.GetInformerServiceFunc().GetSpace(spaceName)
}

func getRolesFromNSTemplateTier(nstemplatetier *toolchainv1alpha1.NSTemplateTier) []string {
	roles := make([]string, 0, len(nstemplatetier.Spec.SpaceRoles))
	for k := range nstemplatetier.Spec.SpaceRoles {
		roles = append(roles, k)
	}
	sort.Strings(roles)
	return roles
}

func listWorkspaceResponse(ctx echo.Context, workspaces []toolchainv1alpha1.Workspace) error {
	workspaceList := &toolchainv1alpha1.WorkspaceList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "WorkspaceList",
			APIVersion: "toolchain.dev.openshift.com/v1alpha1",
		},
		Items: workspaces,
	}

	ctx.Response().Writer.Header().Set("Content-Type", "application/json")
	ctx.Response().Writer.WriteHeader(http.StatusOK)
	return json.NewEncoder(ctx.Response().Writer).Encode(workspaceList)
}

func getWorkspaceResponse(ctx echo.Context, workspace *toolchainv1alpha1.Workspace) error {
	ctx.Response().Writer.Header().Set("Content-Type", "application/json")
	ctx.Response().Writer.WriteHeader(http.StatusOK)
	return json.NewEncoder(ctx.Response().Writer).Encode(workspace)
}

func errorResponse(ctx echo.Context, err *apierrors.StatusError) error {
	ctx.Logger().Error(errs.Wrap(err, "workspace list error"))
	ctx.Response().Writer.Header().Set("Content-Type", "application/json")
	ctx.Response().Writer.WriteHeader(int(err.ErrStatus.Code))
	return json.NewEncoder(ctx.Response().Writer).Encode(err.ErrStatus)
}

// filterUserSpaceBindings returns all the spacebindings for a given username
func filterUserSpaceBindings(username string, allSpaceBindings []toolchainv1alpha1.SpaceBinding) (outputBindings []toolchainv1alpha1.SpaceBinding) {
	for _, binding := range allSpaceBindings {
		if binding.Spec.MasterUserRecord == username {
			outputBindings = append(outputBindings, binding)
		}
	}
	return outputBindings
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
		if SBRName, isSBRBinding := spaceBinding.GetLabels()[toolchainv1alpha1.SpaceBindingRequestLabelKey]; isSBRBinding {
			if SBRName == "" {
				// small corner case where the SBR name for some reason is not generated
				return nil, fmt.Errorf("SpaceBindingRequest name not found on binding: %s", spaceBinding.GetName())
			}
			// this is a binding that was created via SpaceBindingRequest, it can be updated or deleted
			binding.AvailableActions = []string{UpdateBindingAction, DeleteBindingAction}
		} else {
			// this is a binding that was inherited from a parent space,
			// it can only be overridden by another spacebinding containing the same MUR but different role.
			binding.AvailableActions = []string{OverrideBindingAction}
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
