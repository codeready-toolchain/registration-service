package controller

import (
	"log"
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/codeready-toolchain/registration-service/pkg/middleware"
	"github.com/codeready-toolchain/registration-service/pkg/signup"

	"github.com/gin-gonic/gin"
)

// Signup implements the signup endpoint, which is invoked for new user registrations.
type Signup struct {
	logger        *log.Logger
	signupService signup.Service
}

// NewSignup returns a new Signup controller instance.
func NewSignup(logger *log.Logger, signupService signup.Service) *Signup {
	sc := &Signup{
		logger:        logger,
		signupService: signupService,
	}
	return sc
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
	userID := ctx.GetString(middleware.SubKey)
	signupResource, err := s.signupService.GetUserSignup(userID)
	if err != nil {
		s.logger.Println("error getting UserSignup resource", err.Error())
		errors.AbortWithError(ctx, http.StatusInternalServerError, err, "error getting UserSignup resource")
	}
	if signupResource == nil {
		s.logger.Printf("UserSignup resource for userID: %s resource not found", userID)
		ctx.AbortWithStatus(http.StatusNotFound)
	} else {
		ctx.JSON(http.StatusOK, signupResource)
	}
}
