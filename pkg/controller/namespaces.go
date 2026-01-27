package controller

import (
	"errors"
	"net/http"

	customCtx "github.com/codeready-toolchain/registration-service/pkg/context"
	crterrors "github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/pkg/namespaces"
	"github.com/gin-gonic/gin"
)

// NamespacesController holds the required controllers to be able to manage user namespaces.
type NamespacesController interface {
	// ResetNamespaces deletes the user's namespaces so that the appropriate controllers can recreate them.
	ResetNamespaces(*gin.Context)
}

type namespacesCtrl struct {
	namespacesManager namespaces.Manager
}

// NamespaceResetError is a struct with a static error message that will be returned to the user. The goal for this is to not
// leak any internal details to the user in case an error occurs.
type NamespaceResetError struct{}

func (ue *NamespaceResetError) Error() string {
	return "unable to reset namespaces"
}

// NewNamespacesController creates a new instance of the web framework's controller that manages the user's namespaces.
func NewNamespacesController(namespacesManager namespaces.Manager) NamespacesController {
	return &namespacesCtrl{
		namespacesManager: namespacesManager,
	}
}

func (ctrl *namespacesCtrl) ResetNamespaces(ctx *gin.Context) {
	err := ctrl.namespacesManager.ResetNamespaces(ctx)
	if err != nil {
		log.Errorf(ctx, err, `unable to reset the namespaces for user "%s"`, ctx.GetString(customCtx.UsernameKey))

		if errors.As(err, &namespaces.ErrUserSignUpNotFoundDeactivated{}) {
			crterrors.AbortWithError(ctx, http.StatusNotFound, &NamespaceResetError{}, "The user is either not found or deactivated. Please contact the Developer Sandbox team at developersandbox@redhat.com for assistance")
		} else if errors.As(err, &namespaces.ErrUserHasNoProvisionedNamespaces{}) {
			crterrors.AbortWithError(ctx, http.StatusBadRequest, &NamespaceResetError{}, "No namespaces provisioned, unable to perform reset. Please try again in a while and if the issue persists, please contact the Developer Sandbox team at developersandbox@redhat.com for assistance")
		} else {
			crterrors.AbortWithError(ctx, http.StatusInternalServerError, &NamespaceResetError{}, "Unable to reset your namespaces. Please try again in a while and if the issue persists, please contact the Developer Sandbox team at developersandbox@redhat.com for assistance")
		}
		return
	}

	log.Infof(ctx, `namespaces reset initiated for user "%s"`, ctx.GetString(customCtx.UsernameKey))

	ctx.Status(http.StatusAccepted)
	ctx.Writer.WriteHeaderNow()
}
