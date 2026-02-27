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

// ErrNamespaceReset represents the static error message that will be returned
// to the user. The goal for this is to not leak any internal information to
// the user in case an error occurs.
var ErrNamespaceReset = errors.New("namespace reset error")

// NamespacesController holds the required controllers to be able to manage user namespaces.
type NamespacesController interface {
	// ResetNamespaces deletes the user's namespaces so that the appropriate controllers can recreate them.
	ResetNamespaces(*gin.Context)
}

type namespacesCtrl struct {
	namespacesManager namespaces.Manager
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

		if errors.Is(err, namespaces.ErrUserSignUpNotFoundOrDeactivated) {
			crterrors.AbortWithError(ctx, http.StatusNotFound, ErrNamespaceReset, "The user is either not found or deactivated. Please contact the Developer Sandbox team at devsandbox@redhat.com for assistance")
		} else if errors.As(err, &namespaces.ErrUserHasNoProvisionedNamespaces{}) {
			crterrors.AbortWithError(ctx, http.StatusBadRequest, ErrNamespaceReset, "No namespaces provisioned, unable to perform reset. Please try again in a while and if the issue persists, please contact the Developer Sandbox team at devsandbox@redhat.com for assistance")
		} else {
			crterrors.AbortWithError(ctx, http.StatusInternalServerError, ErrNamespaceReset, "Unable to reset your namespaces. Please try again in a while and if the issue persists, please contact the Developer Sandbox team at devsandbox@redhat.com for assistance")
		}
		return
	}

	log.Infof(ctx, `namespaces reset initiated for user "%s"`, ctx.GetString(customCtx.UsernameKey))

	ctx.Status(http.StatusAccepted)
	ctx.Writer.WriteHeaderNow()
}
