package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	errs "github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/metrics"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
)

func HandleSpaceListRequest(spaceLister *SpaceLister) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		// list all user workspaces
		requestReceivedTime := ctx.Get(context.RequestReceivedTime).(time.Time)
		workspaces, err := ListUserWorkspaces(ctx, spaceLister)
		if err != nil {
			spaceLister.ProxyMetrics.RegServWorkspaceHistogramVec.WithLabelValues(fmt.Sprintf("%d", http.StatusInternalServerError), metrics.MetricsLabelVerbList).Observe(time.Since(requestReceivedTime).Seconds()) // using list as the default value for verb to minimize label combinations for prometheus to process
			return errorResponse(ctx, apierrors.NewInternalError(err))
		}
		spaceLister.ProxyMetrics.RegServWorkspaceHistogramVec.WithLabelValues(fmt.Sprintf("%d", http.StatusOK), metrics.MetricsLabelVerbList).Observe(time.Since(requestReceivedTime).Seconds())
		return listWorkspaceResponse(ctx, workspaces)
	}
}

// ListUserWorkspaces returns a list of Workspaces for the current user.
// The function lists all SpaceBindings for the user and return all the workspaces found from this list.
func ListUserWorkspaces(ctx echo.Context, spaceLister *SpaceLister) ([]toolchainv1alpha1.Workspace, error) {
	signup, err := spaceLister.GetProvisionedUserSignup(ctx)
	if err != nil {
		return nil, err
	}
	// signup is not ready
	if signup == nil {
		return []toolchainv1alpha1.Workspace{}, nil
	}

	// get MUR Names
	murNames := getMURNamesForList(ctx, signup)
	if len(murNames) == 0 {
		return []toolchainv1alpha1.Workspace{}, nil
	}

	// get all spacebindings with given mur since no workspace was provided
	spaceBindings, err := listSpaceBindingsForUsers(spaceLister, murNames)
	if err != nil {
		ctx.Logger().Error(errs.Wrap(err, "error listing space bindings"))
		return nil, err
	}

	return workspacesFromSpaceBindings(ctx, spaceLister, signup.Name, spaceBindings), nil
}

// getMURNamesForList returns a list of MasterUserRecord names to use for listing Workspaces.
// If PublicViewer is enabled, the list will also contain the PublicViewer username.
func getMURNamesForList(ctx echo.Context, signup *signup.Signup) []string {
	names := []string{}
	if signup != nil && signup.CompliantUsername != "" {
		names = append(names, signup.CompliantUsername)
	}
	if context.IsPublicViewerEnabled(ctx) {
		names = append(names, toolchainv1alpha1.KubesawAuthenticatedUsername)
	}
	return names
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

func listSpaceBindingsForUsers(spaceLister *SpaceLister, murNames []string) ([]toolchainv1alpha1.SpaceBinding, error) {
	murSelector, err := labels.NewRequirement(toolchainv1alpha1.SpaceBindingMasterUserRecordLabelKey, selection.In, murNames)
	if err != nil {
		return nil, err
	}
	return spaceLister.GetInformerServiceFunc().ListSpaceBindings(*murSelector)
}

func workspacesFromSpaceBindings(ctx echo.Context, spaceLister *SpaceLister, signupName string, spaceBindings []toolchainv1alpha1.SpaceBinding) []toolchainv1alpha1.Workspace {
	workspaces := []toolchainv1alpha1.Workspace{}
	for i := range spaceBindings {
		spacebinding := &spaceBindings[i]
		space, err := getSpace(spaceLister, spacebinding)
		if err != nil {
			// log error and continue so that the api behaves in a best effort manner
			// ie. if a space isn't listed something went wrong but we still want to return the other spaces if possible
			ctx.Logger().Error(nil, err, "unable to get space", "space", spacebinding.Labels[toolchainv1alpha1.SpaceBindingSpaceLabelKey])
			continue
		}
		workspace := createWorkspaceObject(signupName, space, spacebinding)
		workspaces = append(workspaces, *workspace)
	}
	return workspaces
}

func getSpace(spaceLister *SpaceLister, spaceBinding *toolchainv1alpha1.SpaceBinding) (*toolchainv1alpha1.Space, error) {
	spaceName := spaceBinding.Labels[toolchainv1alpha1.SpaceBindingSpaceLabelKey]
	if spaceName == "" { // space may not be initialized
		// log error and continue so that the api behaves in a best effort manner
		return nil, fmt.Errorf("spacebinding has no '%s' label", toolchainv1alpha1.SpaceBindingSpaceLabelKey)
	}
	return spaceLister.GetInformerServiceFunc().GetSpace(spaceName)
}
