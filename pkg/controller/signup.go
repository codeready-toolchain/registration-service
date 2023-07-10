package controller

import (
	"errors"
	"net/http"
	"strconv"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/application"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	crterrors "github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/codeready-toolchain/registration-service/pkg/log"

	"github.com/gin-gonic/gin"
	"github.com/nyaruka/phonenumbers"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// Signup implements the signup endpoint, which is invoked for new user registrations.
type Signup struct {
	app application.Application
}

type Phone struct {
	CountryCode string `form:"country_code" json:"country_code" binding:"required"`
	PhoneNumber string `form:"phone_number" json:"phone_number" binding:"required"`
}

// NewSignup returns a new Signup instance.
func NewSignup(app application.Application) *Signup {
	return &Signup{
		app: app,
	}
}

// PostHandler creates a Signup resource
func (s *Signup) PostHandler(ctx *gin.Context) {
	userSignup, err := s.app.SignupService().Signup(ctx)
	e := &apierrors.StatusError{}
	if errors.As(err, &e) {
		crterrors.AbortWithError(ctx, int(e.Status().Code), err, "error creating UserSignup resource")
		return
	}
	if err != nil {
		log.Error(ctx, err, "error creating UserSignup resource")
		crterrors.AbortWithError(ctx, http.StatusInternalServerError, err, "error creating UserSignup resource")
		return
	}
	if _, exists := userSignup.Annotations[toolchainv1alpha1.UserSignupActivationCounterAnnotationKey]; !exists {
		log.Infof(ctx, "UserSignup created: %s", userSignup.Name)
	} else {
		log.Infof(ctx, "UserSignup reactivated: %s", userSignup.Name)
	}
	ctx.Status(http.StatusAccepted)
	ctx.Writer.WriteHeaderNow()
}

// InitVerificationHandler starts the phone verification process for a user.  It extracts the user's identifying
// information from their Access Token (presented in the Authorization HTTP header) to determine the user, and then
// invokes the Verification service with an E.164 formatted phone number value derived from the country code and phone number
// provided by the user.
func (s *Signup) InitVerificationHandler(ctx *gin.Context) {
	userID := ctx.GetString(context.SubKey)
	username := ctx.GetString(context.UsernameKey)

	// Read the Body content
	var phone Phone
	if err := ctx.BindJSON(&phone); err != nil {
		log.Errorf(ctx, err, "request body does not contain required fields phone_number and country_code")
		crterrors.AbortWithError(ctx, http.StatusBadRequest, err, "error reading request body")
		return
	}

	countryCode, err := strconv.Atoi(phone.CountryCode)
	if err != nil {
		log.Errorf(ctx, err, "invalid country_code value")
		crterrors.AbortWithError(ctx, http.StatusBadRequest, err, "invalid country_code")
		return
	}

	regionCode := phonenumbers.GetRegionCodeForCountryCode(countryCode)
	number, err := phonenumbers.Parse(phone.PhoneNumber, regionCode)
	if err != nil {
		log.Errorf(ctx, err, "invalid phone number")
		crterrors.AbortWithError(ctx, http.StatusBadRequest, err, "invalid phone number provided")
		return
	}

	e164Number := phonenumbers.Format(number, phonenumbers.E164)
	err = s.app.VerificationService().InitVerification(ctx, userID, username, e164Number)
	if err != nil {
		log.Errorf(ctx, err, "Verification for %s could not be sent", userID)
		e := &crterrors.Error{}
		switch {
		case errors.As(err, &e):
			crterrors.AbortWithError(ctx, int(e.Code), err, e.Message)
		default:
			crterrors.AbortWithError(ctx, http.StatusInternalServerError, err, "error while initiating verification")
		}
		return
	}

	log.Infof(ctx, "phone verification has been sent for userID %s", userID)
	ctx.Status(http.StatusNoContent)
	ctx.Writer.WriteHeaderNow()
}

// GetHandler returns the Signup resource
func (s *Signup) GetHandler(ctx *gin.Context) {

	// Get the UserSignup resource from the service by the userID
	userID := ctx.GetString(context.SubKey)
	username := ctx.GetString(context.UsernameKey)
	signupResource, err := s.app.SignupService().GetSignup(ctx, userID, username)
	if err != nil {
		log.Error(ctx, err, "error getting UserSignup resource")
		crterrors.AbortWithError(ctx, http.StatusInternalServerError, err, "error getting UserSignup resource")
	}
	if signupResource == nil {
		log.Infof(ctx, "UserSignup resource for userID: %s, username: %s resource not found", userID, username)
		ctx.AbortWithStatus(http.StatusNotFound)
	} else {
		ctx.JSON(http.StatusOK, signupResource)
	}
}

// VerifyPhoneCodeHandler validates the phone verification code passed in by the user
func (s *Signup) VerifyPhoneCodeHandler(ctx *gin.Context) {
	code := ctx.Param("code")
	if code == "" {
		log.Error(ctx, nil, "no phone code provided in the request")
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}

	userID := ctx.GetString(context.SubKey)
	username := ctx.GetString(context.UsernameKey)

	err := s.app.VerificationService().VerifyPhoneCode(ctx, userID, username, code)
	if err != nil {
		log.Error(ctx, err, "error validating user verification phone code")
		e := &crterrors.Error{}
		switch {
		case errors.As(err, &e):
			crterrors.AbortWithError(ctx, int(e.Code), err, "error while verifying phone code")
		default:
			crterrors.AbortWithError(ctx, http.StatusInternalServerError, err, "unexpected error while verifying phone code")
		}
		return
	}
	ctx.Status(http.StatusOK)
}

// VerifyActivationCodeHandler validates the activation code passed in by the user as a form value
func (s *Signup) VerifyActivationCodeHandler(ctx *gin.Context) {
	body := map[string]interface{}{}
	if err := ctx.BindJSON(&body); err != nil {
		log.Error(ctx, nil, "no activation code provided in the request")
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}
	code, ok := body["code"].(string)
	if !ok {
		log.Error(ctx, nil, "no activation code provided in the request")
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}

	userID := ctx.GetString(context.SubKey)
	username := ctx.GetString(context.UsernameKey)

	err := s.app.VerificationService().VerifyActivationCode(ctx, userID, username, code)
	if err != nil {
		log.Error(ctx, err, "error validating activation code")
		e := &crterrors.Error{}
		switch {
		case errors.As(err, &e):
			crterrors.AbortWithError(ctx, int(e.Code), err, "error while verifying activation code")
		default:
			crterrors.AbortWithError(ctx, http.StatusInternalServerError, err, "unexpected error while verifying activation code")
		}
		return
	}
	ctx.Status(http.StatusOK)
}
