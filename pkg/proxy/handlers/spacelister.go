package handlers

import (
	"encoding/json"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/application"
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/metrics"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	commonproxy "github.com/codeready-toolchain/toolchain-common/pkg/proxy"
	"github.com/gin-gonic/gin"

	"github.com/labstack/echo/v4"
	errs "github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	ProxyMetrics           *metrics.ProxyMetrics
}

func NewSpaceLister(app application.Application, proxyMetrics *metrics.ProxyMetrics) *SpaceLister {
	return &SpaceLister{
		GetSignupFunc:          app.SignupService().GetSignupFromInformer,
		GetInformerServiceFunc: app.InformerService,
		ProxyMetrics:           proxyMetrics,
	}
}

func (s *SpaceLister) GetProvisionedUserSignup(ctx echo.Context) (*signup.Signup, error) {
	userID, _ := ctx.Get(context.SubKey).(string)
	username, _ := ctx.Get(context.UsernameKey).(string)

	userSignup, err := s.GetSignupFunc(nil, userID, username, false)
	if err != nil {
		ctx.Logger().Error(errs.Wrap(err, "error retrieving signup"))
		return nil, err
	}
	if userSignup == nil || userSignup.CompliantUsername == "" {
		// account exists but the compliant username is not set yet, meaning it has not been fully provisioned yet, so return an empty list
		return nil, nil
	}
	return userSignup, nil
}

func createWorkspaceObject(signupName *string, space *toolchainv1alpha1.Space, spaceBinding *toolchainv1alpha1.SpaceBinding, wsAdditionalOptions ...commonproxy.WorkspaceOption) *toolchainv1alpha1.Workspace {
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
	if signupName != nil && ownerName == *signupName {
		wsOptions = append(wsOptions, commonproxy.WithType("home"))
	}
	wsOptions = append(wsOptions, wsAdditionalOptions...)

	workspace := commonproxy.NewWorkspace(space.GetName(), wsOptions...)
	return workspace
}

func errorResponse(ctx echo.Context, err *apierrors.StatusError) error {
	ctx.Logger().Error(errs.Wrap(err, "workspace list error"))
	ctx.Response().Writer.Header().Set("Content-Type", "application/json")
	ctx.Response().Writer.WriteHeader(int(err.ErrStatus.Code))
	return json.NewEncoder(ctx.Response().Writer).Encode(err.ErrStatus)
}
