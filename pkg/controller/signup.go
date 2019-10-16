package controller

import (
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/pkg/signup"

	"github.com/gin-gonic/gin"
)

// Signup implements the signup endpoint, which is invoked for new user registrations.
type Signup struct {
	config        *configuration.Registry
	signupService signup.Service
}

// NewSignup returns a new Signup instance.
func NewSignup(config *configuration.Registry, signupService signup.Service) *Signup {
	return &Signup{
		config:        config,
		signupService: signupService,
	}
}

// PostHandler creates a Signup resource
func (s *Signup) PostHandler(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", "application/json")
	// TODO call s.signupService.CreateUserSignup() to create the actual resource in Kube API Server
	ctx.Writer.WriteHeader(http.StatusOK)
}

// GetHandler returns the Signup resource
func (s *Signup) GetHandler(ctx *gin.Context) {
	// Get the UserSignup resource from the service by the userID
	userID := ctx.GetString(context.SubKey)
	signupResource, err := s.signupService.GetSignup(userID)
	if err != nil {
		log.Error(ctx, err, "error getting UserSignup resource")
		errors.AbortWithError(ctx, http.StatusInternalServerError, err, "error getting UserSignup resource")
	}
	if signupResource == nil {
		log.Errorf(ctx, nil, "UserSignup resource for userID: %s resource not found", userID)
		ctx.AbortWithStatus(http.StatusNotFound)
	} else {
		ctx.JSON(http.StatusOK, signupResource)
	}
}
