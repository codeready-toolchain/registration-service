package controller

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/codeready-toolchain/registration-service/pkg/middleware"
	"github.com/codeready-toolchain/registration-service/pkg/signup"

	"github.com/gin-gonic/gin"
)

// Signup implements the signup endpoint, which is invoked for new user registrations.
type Signup struct {
	config        *configuration.Registry
	logger        *log.Logger
	signupService signup.SignupService
}

// NewSignup returns a new Signup controller instance.
func NewSignup(logger *log.Logger, config *configuration.Registry, signupService signup.SignupService) *Signup {
	sc := &Signup{
		logger:        logger,
		config:        config,
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
	ctx.Writer.Header().Set("Content-Type", "application/json")
	// Get the UserSignup resource from the service by the userID
	signupResource, err := s.signupService.GetUserSignup(ctx.GetString(middleware.SubKey))
	if err != nil {
		s.logger.Println("error getting UserSignup resource", err.Error())
		errors.EncodeError(ctx, err, http.StatusInternalServerError, "error getting UserSignup resource")
	}
	if signupResource == nil {
		ctx.Writer.WriteHeader(http.StatusNotFound)
	} else {
		ctx.Writer.WriteHeader(http.StatusOK)
		err := json.NewEncoder(ctx.Writer).Encode(signupResource)
		if err != nil {
			s.logger.Println("error writing response body", err.Error())
			errors.EncodeError(ctx, err, http.StatusInternalServerError, "error writing response body")
		}
	}
}
