package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/application"
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	commonproxy "github.com/codeready-toolchain/toolchain-common/pkg/proxy"
	"github.com/gin-gonic/gin"

	"github.com/labstack/echo/v4"
	errs "github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
)

type SpaceLister struct {
	GetSignupFunc          func(ctx *gin.Context, userID, username string, checkUserSignupCompleted bool) (*signup.Signup, error)
	GetNSTemplateTierFunc  func(ctx *gin.Context, tier string) (*toolchainv1alpha1.NSTemplateTier, error)
	GetInformerServiceFunc func() service.InformerService
}

func NewSpaceLister(app application.Application) *SpaceLister {
	return &SpaceLister{
		GetSignupFunc:          app.SignupService().GetSignupFromInformer,
		GetInformerServiceFunc: app.InformerService,
	}
}

func (s *SpaceLister) HandleSpaceListRequest(ctx echo.Context) error {
	userID, _ := ctx.Get(context.SubKey).(string)
	username, _ := ctx.Get(context.UsernameKey).(string)

	userSignup, err := s.GetSignupFunc(nil, userID, username, false)
	if err != nil {
		ctx.Logger().Error(errs.Wrap(err, "error retrieving userSignup"))
		return errorResponse(ctx, apierrors.NewInternalError(err))
	}
	if userSignup == nil || userSignup.CompliantUsername == "" {
		// account exists but the compliant username is not set yet, meaning it has not been fully provisioned yet
		// return empty workspace list
		return listWorkspaceResponse(ctx, []toolchainv1alpha1.Workspace{})
	}

	// list all user workspaces
	workspaces, err := s.ListUserWorkspaces(ctx)
	if err != nil {
		return errorResponse(ctx, apierrors.NewInternalError(err))
	}
	return listWorkspaceResponse(ctx, workspaces)
}

func (s *SpaceLister) HandleSpaceGetRequest(ctx echo.Context) error {
	userID, _ := ctx.Get(context.SubKey).(string)
	username, _ := ctx.Get(context.UsernameKey).(string)
	workspaceName := ctx.Param("workspace")

	userSignup, err := s.GetSignupFunc(nil, userID, username, false)
	if err != nil {
		ctx.Logger().Error(errs.Wrap(err, "error retrieving userSignup"))
		return errorResponse(ctx, apierrors.NewInternalError(err))
	}
	if userSignup == nil || userSignup.CompliantUsername == "" {
		// account exists but the compliant username is not set yet, meaning it has not been fully provisioned yet
		// return not found response when specific workspace request was issued
		r := schema.GroupResource{Group: "toolchain.dev.openshift.com", Resource: "workspaces"}
		return errorResponse(ctx, apierrors.NewNotFound(r, workspaceName))

	}

	// get specific workspace
	workspace, err := s.GetUserWorkspace(ctx, userSignup)
	if err != nil {
		return errorResponse(ctx, apierrors.NewInternalError(err))
	}
	if workspace == nil {
		// not found
		r := schema.GroupResource{Group: "toolchain.dev.openshift.com", Resource: "workspaces"}
		return errorResponse(ctx, apierrors.NewNotFound(r, workspaceName))
	}
	return getWorkspaceResponse(ctx, workspace)
}

func (s *SpaceLister) GetUserWorkspace(ctx echo.Context, signup *signup.Signup) (*toolchainv1alpha1.Workspace, error) {
	murName := signup.CompliantUsername
	spaceBinding, err := s.listSpaceBindingForUserAndSpace(ctx, murName)
	if err != nil {
		ctx.Logger().Error(errs.Wrap(err, "error listing space bindings"))
		return nil, err
	}
	if spaceBinding == nil {
		// spacebinding not found, let's return a nil workspace which causes the handler to respond with a 404 status code
		return nil, nil
	}

	space, err := s.getSpace(spaceBinding)
	if err != nil {
		ctx.Logger().Error(errs.Wrap(err, "unable to get space"))
		return nil, err
	}

	// -------------
	// TODO recursively get all the spacebindings for the current workspace
	// and build the Bindings list with the available actions
	// this field is populated only for the GET workspace request
	// -------------

	// add available roles, this field is populated only for the GET workspace request
	nsTemplateTier, err := s.GetNSTemplateTierFunc(nil, space.Spec.TierName)
	if err != nil {
		ctx.Logger().Error(errs.Wrap(err, "unable to get nstemplatetier"))
		return nil, err
	}
	getOnlyWSOptions := commonproxy.WithAvailableRoles(getRolesFromNSTemplateTier(nsTemplateTier))

	return createWorkspaceObject(signup.Name, space, spaceBinding, getOnlyWSOptions), nil
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

func (s *SpaceLister) listSpaceBindingForUserAndSpace(ctx echo.Context, murName string) (*toolchainv1alpha1.SpaceBinding, error) {
	workspaceName := ctx.Param("workspace")
	murSelector, err := labels.NewRequirement(toolchainv1alpha1.SpaceBindingMasterUserRecordLabelKey, selection.Equals, []string{murName})
	if err != nil {
		return nil, err
	}
	// specific workspace requested so add label requirement to match the space
	spaceSelector, err := labels.NewRequirement(toolchainv1alpha1.SpaceBindingSpaceLabelKey, selection.Equals, []string{workspaceName})
	if err != nil {
		return nil, err
	}
	requirements := []labels.Requirement{*murSelector, *spaceSelector}

	spaceBindings, err := s.GetInformerServiceFunc().ListSpaceBindings(requirements...)
	if err != nil {
		return nil, err
	}

	//  let's only log the issue and consider this as not found
	if len(spaceBindings) != 1 {
		ctx.Logger().Error("expected only 1 spacebinding, got %d for user %s and workspace %s", len(spaceBindings), murName, workspaceName)
		return nil, nil
	}

	return &spaceBindings[0], nil
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
	roles := make([]string, len(nstemplatetier.Spec.SpaceRoles))
	i := 0
	for k := range nstemplatetier.Spec.SpaceRoles {
		roles[i] = k
		i++
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
