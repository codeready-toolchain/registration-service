package controller

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/middleware"
	"github.com/codeready-toolchain/registration-service/pkg/signup"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/gin-gonic/gin"
)

// Signup implements the signup endpoint, which is invoked for new user registrations.
type Signup struct {
	config        *configuration.Registry
	logger        *log.Logger
	signupService signup.SignupService
}

// NewSignup returns a new Signup instance.
func NewSignup(logger *log.Logger, config *configuration.Registry) (*Signup, error) {
	signupService, err := signup.NewSignupService(config)
	if err != nil {
		logger.Printf("error creating SignupService", err)
		return nil, err
	}

	return &Signup{
		logger:        logger,
		config:        config,
		signupService: signupService,
	}, nil
}

// PostHandler returns signup info.
func (s *Signup) PostHandler(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", "application/json")

	userSignup, err := s.signupService.CreateUserSignup(ctx, ctx.GetString(middleware.UsernameKey), ctx.GetString(middleware.SubKey))
	if err != nil {

	}

	s.logger.Printf("UserSignup %s created", userSignup.Name)

	err = json.NewEncoder(ctx.Writer).Encode(nil)
	if err != nil {
		s.logger.Println("error writing response body", err.Error())
		errors.EncodeError(ctx, err, http.StatusInternalServerError, "error writing response body")
	}
}
