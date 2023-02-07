package handlers

import (
	"encoding/json"
	"net/http"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/application"
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/pkg/signup"

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
	userID, _ := ctx.Get(context.SubKey).(string)
	username, _ := ctx.Get(context.UsernameKey).(string)
	workspaceName := ctx.Param("workspace")
	doGetWorkspace := len(workspaceName) > 0

	signup, err := s.GetSignupFunc(userID, username)
	if err != nil {
		return err
	}
	if signup == nil || !signup.Status.Ready {
		return errs.New("user is not provisioned (yet)")
	}

	murName := signup.CompliantUsername
	murSelector, err := labels.NewRequirement(toolchainv1alpha1.SpaceBindingMasterUserRecordLabelKey, selection.Equals, []string{murName})
	if err != nil {
		return err
	}

	requirements := []labels.Requirement{*murSelector}

	if doGetWorkspace {
		// specific workspace requested so add label requirement to match the space
		spaceSelector, err := labels.NewRequirement(toolchainv1alpha1.SpaceBindingSpaceLabelKey, selection.Equals, []string{workspaceName})
		if err != nil {
			return err
		}
		requirements = append(requirements, *spaceSelector)
	}

	spaceBindings, err := s.GetInformerServiceFunc().ListSpaceBindings(requirements...)
	if err != nil {
		return err
	}

	workspaces, err := s.workspacesFromSpaceBindings(spaceBindings)
	if err != nil {
		return err
	}

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
	ctx.Logger().Error(err, err.Error())
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

func (s *SpaceLister) workspacesFromSpaceBindings(spaceBindings []*toolchainv1alpha1.SpaceBinding) ([]toolchainv1alpha1.Workspace, error) {
	workspaces := []toolchainv1alpha1.Workspace{}
	for _, spaceBinding := range spaceBindings {

		if spaceBinding.Labels == nil {
			continue
		}

		var ownerName string
		spaceName := spaceBinding.Labels[toolchainv1alpha1.SpaceBindingSpaceLabelKey]
		space, err := s.GetInformerServiceFunc().GetSpace(spaceName)
		if err != nil {
			// log error and continue so that the api behaves in a best effort manner
			// ie. if a space isn't listed something went wrong but we still want to return the other spaces if possible
			log.Errorf(nil, err, "unable to get space", "space", spaceName)
			continue
		}
		if space.Labels != nil { // space may not be initialized yet
			// TODO right now we get SpaceCreatorLabelKey but should get owner from Space once it's implemented
			ownerName = space.Labels[toolchainv1alpha1.SpaceCreatorLabelKey]
			if spaceName == "" { // space may not be initialized
				continue
			}
		}

		// TODO get namespaces from Space status once it's implemented
		namespaces := []toolchainv1alpha1.SpaceNamespace{
			{
				Name: spaceName + "-tenant",
				Type: "default",
			},
		}

		workspace := NewWorkspace(spaceName,
			WithNamespaces(namespaces),
			WithOwner(ownerName),
			WithRole(spaceBinding.Spec.SpaceRole),
		)
		workspaces = append(workspaces, *workspace)
	}
	return workspaces, nil
}
