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
	userID, _ := ctx.Get(context.SubKey).(string)
	username, _ := ctx.Get(context.UsernameKey).(string)
	workspaceName := ctx.Param("workspace")
	doGetWorkspace := len(workspaceName) > 0

	signup, err := s.GetSignupFunc(userID, username)
	if err != nil {
		log.Error(nil, err, "error retrieving signup")
		return errorResponse(ctx, apierrors.NewInternalError(fmt.Errorf("user account lookup failed")), http.StatusInternalServerError)
	}
	if signup == nil || !signup.Status.Ready {
		return errorResponse(ctx, apierrors.NewUnauthorized("user account not ready"), http.StatusUnauthorized)
	}

	murName := signup.CompliantUsername
	murSelector, err := labels.NewRequirement(toolchainv1alpha1.SpaceBindingMasterUserRecordLabelKey, selection.Equals, []string{murName})
	if err != nil {
		log.Error(nil, err, "error constructing mur selector")
		return errorResponse(ctx, apierrors.NewInternalError(fmt.Errorf("user account lookup failed")), http.StatusInternalServerError)
	}

	requirements := []labels.Requirement{*murSelector}

	if doGetWorkspace {
		// specific workspace requested so add label requirement to match the space
		spaceSelector, err := labels.NewRequirement(toolchainv1alpha1.SpaceBindingSpaceLabelKey, selection.Equals, []string{workspaceName})
		if err != nil {
			log.Error(nil, err, "error constructing space selector")
			return errorResponse(ctx, apierrors.NewInternalError(fmt.Errorf("workspace lookup failed")), http.StatusInternalServerError)
		}
		requirements = append(requirements, *spaceSelector)
	}

	spaceBindings, err := s.GetInformerServiceFunc().ListSpaceBindings(requirements...)
	if err != nil {
		log.Error(nil, err, "error listing space bindings")
		return errorResponse(ctx, apierrors.NewInternalError(fmt.Errorf("workspace lookup failed")), http.StatusInternalServerError)
	}

	workspaces := s.workspacesFromSpaceBindings(spaceBindings)

	if doGetWorkspace { // specific workspace requested
		if len(workspaces) != 1 {
			// not found
			r := schema.GroupResource{Group: "toolchain.dev.openshift.com", Resource: "workspaces"}
			return errorResponse(ctx, apierrors.NewNotFound(r, workspaceName), http.StatusNotFound)
		}
		return getWorkspaceResponse(ctx, workspaces[0])
	}

	return listWorkspaceResponse(ctx, workspaces)
}

func errorResponse(ctx echo.Context, err *apierrors.StatusError, code int) error {
	ctx.Logger().Error(err, "workspace list error")
	ctx.Response().Writer.Header().Set("Content-Type", "application/json")
	ctx.Response().Writer.WriteHeader(code)
	return json.NewEncoder(ctx.Response().Writer).Encode(err.ErrStatus)
}

func getWorkspaceResponse(ctx echo.Context, workspace toolchainv1alpha1.Workspace) error {
	ctx.Response().Writer.Header().Set("Content-Type", "application/json")
	ctx.Response().Writer.WriteHeader(http.StatusOK)
	return json.NewEncoder(ctx.Response().Writer).Encode(workspace)
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

func (s *SpaceLister) workspacesFromSpaceBindings(spaceBindings []*toolchainv1alpha1.SpaceBinding) []toolchainv1alpha1.Workspace {
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

		workspace := commonproxy.NewWorkspace(spaceName,
			commonproxy.WithNamespaces(namespaces),
			commonproxy.WithOwner(ownerName),
			commonproxy.WithRole(spaceBinding.Spec.SpaceRole),
		)
		workspaces = append(workspaces, *workspace)
	}
	return workspaces
}
