package controller

import (
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/verification"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/pkg/signup"

	"github.com/gin-gonic/gin"
)

// Signup implements the signup endpoint, which is invoked for new user registrations.
type Signup struct {
	config              *configuration.Registry
	signupService       signup.Service
	verificationService verification.Service
}

// NewSignup returns a new Signup instance.
func NewSignup(config *configuration.Registry, signupService signup.Service, verificationService verification.Service) *Signup {
	return &Signup{
		config:              config,
		signupService:       signupService,
		verificationService: verificationService,
	}
}

// PostHandler creates a Signup resource
func (s *Signup) PostHandler(ctx *gin.Context) {
	userSignup, err := s.signupService.CreateUserSignup(ctx)
	if err != nil {
		log.Error(ctx, err, "error creating UserSignup resource")
		errors.AbortWithError(ctx, http.StatusInternalServerError, err, "error creating UserSignup resource")
		return
	}

	log.Infof(ctx, "UserSignup %s created", userSignup.Name)
	ctx.Status(http.StatusAccepted)
	ctx.Writer.WriteHeaderNow()
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

// VerifyCodeHandler validates the phone verification code passed in by the user
func (s *Signup) VerifyCodeHandler(ctx *gin.Context) {
	code := ctx.Param("code")

	userID := ctx.GetString(context.SubKey)
	signupResource, err := s.signupService.GetUserSignup(userID)
	if err != nil {
		log.Error(ctx, err, "error getting UserSignup resource")
		errors.AbortWithError(ctx, http.StatusInternalServerError, err, "error getting UserSignup resource")
	}
	if signupResource == nil {
		log.Errorf(ctx, nil, "UserSignup resource for userID: %s resource not found", userID)
		ctx.AbortWithStatus(http.StatusNotFound)
	}

	err = s.verificationService.VerifyCode(ctx, signupResource, code)
	if err != nil {
		log.Error(ctx, err, "error validating user verification code")
		// TODO we need to check what type of error is returned here
		//errors.AbortWithError(ctx)
	}

	err = s.signupService.UpdateUserSignup(signupResource)
	if err != nil {
		log.Error(ctx, err, "error while updating UserSignup resource")
		errors.AbortWithError(ctx, http.StatusInternalServerError, err, "error while updating UserSignup resource")
	}

	ctx.Status(http.StatusOK)
}
