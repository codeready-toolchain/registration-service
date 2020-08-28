package controller

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	errors2 "k8s.io/apimachinery/pkg/api/errors"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/registration-service/pkg/verification"

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

// PostVerificationHandler creates a verification and updates a usersignup resource
func (s *Signup) PostVerificationHandler(ctx *gin.Context) {
	userID := ctx.GetString(context.SubKey)

	// Read the Body content
	var bodyBytes []byte
	if ctx.Request.Body != nil {
		bodyBytes, _ = ioutil.ReadAll(ctx.Request.Body)
	}

	m := make(map[string]string)
	err := json.Unmarshal(bodyBytes, &m)
	if err != nil {
		log.Errorf(ctx, nil, "Request body could not be read")
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// generate verification code
	code, err := s.verificationService.GenerateVerificationCode()
	if err != nil {
		log.Errorf(ctx, nil, "verification code could not be generated")
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	userSignup, httpCode, err := s.signupService.PostVerification(s.config.GetVerificationDailyLimit(), m, userID, code)
	if err != nil {
		log.Errorf(ctx, nil, "phone verification has failed: %s", err.Error())
		ctx.AbortWithError(httpCode, err)
		return
	}

	err = s.verificationService.SendVerification(ctx, userSignup)
	if err != nil {
		log.Errorf(ctx, nil, "Verification for %s could not be sent", userID)
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	log.Infof(ctx, "phone verification has passed for userID %s", userID)
	ctx.JSON(http.StatusOK, userSignup)
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
	if code == "" {
		log.Error(ctx, nil, "no code provided in request")
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}

	userID := ctx.GetString(context.SubKey)
	signupResource, err := s.signupService.GetUserSignup(userID)
	if err != nil {
		log.Error(ctx, err, "error getting UserSignup resource")
		errors.AbortWithError(ctx, http.StatusInternalServerError, err, "error getting UserSignup resource")
		return
	}

	if signupResource == nil {
		log.Errorf(ctx, nil, "UserSignup resource for userID: %s resource not found", userID)
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}

	// The VerifyCode() call here MAY make changes to the specified signupResource
	err = s.verificationService.VerifyCode(ctx, signupResource, code)

	// Regardless of whether the VerifyCode() call returns an error or not, we need to update the UserSignup instance
	// as its state can be updated even in the case of an error.  This may result in the slight possibility that any
	// errors returned by VerifyCode() are suppressed, as error handling for the UserSignup update is given precedence.
	_, err2 := s.signupService.UpdateUserSignup(signupResource)
	if err2 != nil {
		log.Error(ctx, err2, "error while updating UserSignup resource")
		errors.AbortWithError(ctx, http.StatusInternalServerError, err2, "error while updating UserSignup resource")

		if err != nil {
			log.Error(ctx, err, "error validating user verification code")
		}
		return
	}

	if err != nil {
		log.Error(ctx, err, "error validating user verification code")
		switch t := err.(type) {
		default:
			errors.AbortWithError(ctx, http.StatusInternalServerError, err, "error while verifying code")
		case *errors2.StatusError:
			errors.AbortWithError(ctx, int(t.ErrStatus.Code), err, t.ErrStatus.Message)
		}
		return
	}

	ctx.Status(http.StatusOK)
}
