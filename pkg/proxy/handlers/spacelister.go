package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/application"
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	commonproxy "github.com/codeready-toolchain/toolchain-common/pkg/proxy"

	"github.com/labstack/echo/v4"
	errs "github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
)

type SpaceLister struct {
	GetSignupFunc          func(userID, username string) (*signup.Signup, error)
	GetInformerServiceFunc func() service.InformerService
}

func NewSpaceLister(app application.Application) *SpaceLister {
	return &SpaceLister{
		GetSignupFunc:          app.SignupService().GetSignupFromInformer,
		GetInformerServiceFunc: app.InformerService,
	}
}

func (s *SpaceLister) HandleSpaceListRequest(ctx echo.Context) error {

	workspaces, err := s.ListUserWorkspaces(ctx)
	if err != nil {
		return errorResponse(ctx, apierrors.NewInternalError(err))
	}

	workspaceName := ctx.Param("workspace")
	doGetWorkspace := len(workspaceName) > 0

	if doGetWorkspace { // specific workspace requested
		if len(workspaces) != 1 {
			// not found
			r := schema.GroupResource{Group: "toolchain.dev.openshift.com", Resource: "workspaces"}
			return errorResponse(ctx, apierrors.NewNotFound(r, workspaceName))
		}
		return getWorkspaceResponse(ctx, workspaces[0])
	}

	return listWorkspaceResponse(ctx, workspaces)
}

func (s *SpaceLister) ListUserWorkspaces(ctx echo.Context) ([]toolchainv1alpha1.Workspace, error) {
	userID, _ := ctx.Get(context.SubKey).(string)
	username, _ := ctx.Get(context.UsernameKey).(string)

	signup, err := s.GetSignupFunc(userID, username)
	if err != nil {
		ctx.Logger().Error(errs.Wrap(err, "error retrieving signup"))
		return nil, err
	}
	if signup == nil || !signup.Status.Ready {
		// account exists but is not ready so return an empty list
		return []toolchainv1alpha1.Workspace{}, nil
	}

	murName := signup.CompliantUsername
	spaceBindings, err := s.listSpaceBindingsForUser(ctx, murName)
	if err != nil {
		ctx.Logger().Error(errs.Wrap(err, "error listing space bindings"))
		return nil, err
	}

	return s.workspacesFromSpaceBindings(signup.Name, spaceBindings), nil
}

func (s *SpaceLister) listSpaceBindingsForUser(ctx echo.Context, murName string) ([]*toolchainv1alpha1.SpaceBinding, error) {

	workspaceName := ctx.Param("workspace")
	doGetWorkspace := len(workspaceName) > 0

	murSelector, err := labels.NewRequirement(toolchainv1alpha1.SpaceBindingMasterUserRecordLabelKey, selection.Equals, []string{murName})
	if err != nil {
		return nil, err
	}

	requirements := []labels.Requirement{*murSelector}

	if doGetWorkspace {
		// specific workspace requested so add label requirement to match the space
		spaceSelector, err := labels.NewRequirement(toolchainv1alpha1.SpaceBindingSpaceLabelKey, selection.Equals, []string{workspaceName})
		if err != nil {
			return nil, err
		}
		requirements = append(requirements, *spaceSelector)
	}

	spaceBindings, err := s.GetInformerServiceFunc().ListSpaceBindings(requirements...)
	if err != nil {
		return nil, err
	}
	return spaceBindings, nil
}

func (s *SpaceLister) workspacesFromSpaceBindings(signupName string, spaceBindings []*toolchainv1alpha1.SpaceBinding) []toolchainv1alpha1.Workspace {
	workspaces := []toolchainv1alpha1.Workspace{}
	for _, spaceBinding := range spaceBindings {

		var ownerName string
		spaceName := spaceBinding.Labels[toolchainv1alpha1.SpaceBindingSpaceLabelKey]
		if spaceName == "" { // space may not be initialized
			// log error and continue so that the api behaves in a best effort manner
			log.Errorf(nil, fmt.Errorf("spacebinding has no '%s' label", toolchainv1alpha1.SpaceBindingSpaceLabelKey), "unable to get space", "space", spaceName)
			continue
		}
		space, err := s.GetInformerServiceFunc().GetSpace(spaceName)
		if err != nil {
			// log error and continue so that the api behaves in a best effort manner
			// ie. if a space isn't listed something went wrong but we still want to return the other spaces if possible
			log.Errorf(nil, err, "unable to get space", "space", spaceName)
			continue
		}

		// TODO right now we get SpaceCreatorLabelKey but should get owner from Space once it's implemented
		ownerName = space.Labels[toolchainv1alpha1.SpaceCreatorLabelKey]

		// TODO get namespaces from Space status once it's implemented
		namespaces := []toolchainv1alpha1.SpaceNamespace{
			{
				Name: spaceName + "-tenant",
				Type: "default",
			},
		}

		wsOptions := []commonproxy.WorkspaceOption{
			commonproxy.WithNamespaces(namespaces),
			commonproxy.WithOwner(ownerName),
			commonproxy.WithRole(spaceBinding.Spec.SpaceRole),
			commonproxy.WithObjectMetaFrom(space.ObjectMeta),
		}
		// set the workspace type to "home" to indicate it is the user's home space
		// TODO set home type based on UserSignup.Status.HomeSpace once it's implemented
		if ownerName == signupName {
			wsOptions = append(wsOptions, commonproxy.WithType("home"))
		}

		workspace := commonproxy.NewWorkspace(spaceName, wsOptions...)
		workspaces = append(workspaces, *workspace)
	}
	return workspaces
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

func getWorkspaceResponse(ctx echo.Context, workspace toolchainv1alpha1.Workspace) error {
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
