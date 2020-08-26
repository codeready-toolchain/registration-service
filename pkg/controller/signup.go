package controller

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

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
	}
	countryCode := m["country_code"]
	phoneNumber := m["phone_number"]

	// generate verification code
	service := verification.NewVerificationService(s.config)
	code, err := service.GenerateVerificationCode()
	if err != nil {
		log.Errorf(ctx, nil, "verification code could not be generated")
		ctx.AbortWithError(http.StatusInternalServerError, err)
	}

	userSignup, httpCode, err := s.signupService.PostVerification(s.config.GetVerificationDailyLimit(), userID, code, countryCode, phoneNumber)
	if err != nil {
		log.Errorf(ctx, nil, "phone verification has failed: %s", err.Error())
		ctx.AbortWithError(httpCode, err)
	}

	err = service.SendVerification(ctx, userSignup)
	if err != nil {
		log.Errorf(ctx, nil, "Verification for %s could not be sent", userID)
		ctx.AbortWithError(http.StatusInternalServerError, err)
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
	}

	ctx.JSON(http.StatusOK, signupResource)

}
