package controller

import (
	"net/http"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	crterrors "github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/pkg/namespaced"
	"github.com/codeready-toolchain/registration-service/pkg/username"
	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/api/errors"
)

// Usernames implements the usernames endpoint, which is invoked for checking if a given username/email exists.
type Usernames struct {
	namespaced.Client
}

// NewUsernames returns a new Usernames instance.
func NewUsernames(nsClient namespaced.Client) *Usernames {
	return &Usernames{
		Client: nsClient,
	}
}

// GetHandler returns the list of usernames found, if any.
func (s *Usernames) GetHandler(ctx *gin.Context) {
	queryString := ctx.Param("username")
	if queryString == "" {
		log.Info(ctx, "empty username provided")
		ctx.AbortWithStatus(http.StatusNotFound)
	}

	// TODO check if the queryString is an email
	// in that case we have to fetch the UserSignup resources with the provided email and the MasterUserRecords associated with those.

	murResource := &toolchainv1alpha1.MasterUserRecord{}
	err := s.Get(ctx.Request.Context(), s.NamespacedName(queryString), murResource)
	// handle not found error
	if errors.IsNotFound(err) {
		log.Infof(ctx, "MasterUserRecord resource for: %s not found", queryString)
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}
	// ...otherwise is a server error
	if err != nil {
		log.Error(ctx, err, "error getting MasterUserRecord resource")
		crterrors.AbortWithError(ctx, http.StatusInternalServerError, err, "error getting MasterUserRecord resource")
		return
	}

	// TODO
	// once we implement search by email the response might contain multiple usernames
	// for now there can be only one username with a given name.
	ctx.JSON(http.StatusOK, username.Response{
		{Username: murResource.GetName()},
	})
}
