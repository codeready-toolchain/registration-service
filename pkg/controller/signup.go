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

	"github.com/labstack/echo/v4"
	"github.com/nyaruka/phonenumbers"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// Signup implements the signup endpoint, which is invoked for new user registrations.
type Signup struct {
	app application.Application
}

type Phone struct {
	CountryCode string `form:"country_code" json:"country_code" validate:"required"`
	PhoneNumber string `form:"phone_number" json:"phone_number" validate:"required"`
}

// NewSignup returns a new Signup instance.
func NewSignup(app application.Application) *Signup {
	return &Signup{
		app: app,
	}
}

// PostHandler creates a Signup resource
func (s *Signup) PostHandler(ctx echo.Context) error {
	userSignup, err := s.app.SignupService().Signup(ctx)
	e := &apierrors.StatusError{}
	if errors.As(err, &e) {
		return crterrors.AbortWithError(ctx, int(e.Status().Code), err, "error creating UserSignup resource")
	}
	if err != nil {
		log.Error(ctx, err, "error creating UserSignup resource")
		return crterrors.AbortWithError(ctx, http.StatusInternalServerError, err, "error creating UserSignup resource")
	}
	if _, exists := userSignup.Annotations[toolchainv1alpha1.UserSignupActivationCounterAnnotationKey]; !exists {
		log.Infof(ctx, "UserSignup created: %s", userSignup.Name)
	} else {
		log.Infof(ctx, "UserSignup reactivated: %s", userSignup.Name)
	}
	return ctx.NoContent(http.StatusAccepted)
}

// InitVerificationHandler starts the phone verification process for a user.  It extracts the user's identifying
// information from their Access Token (presented in the Authorization HTTP header) to determine the user, and then
// invokes the Verification service with an E.164 formatted phone number value derived from the country code and phone number
// provided by the user.
func (s *Signup) InitVerificationHandler(ctx echo.Context) error {
	username := context.GetString(ctx, context.UsernameKey)

	// Read the Body content
	var phone Phone
	if err := ctx.Bind(&phone); err != nil {
		log.Errorf(ctx, err, "request body does not contain required fields phone_number and country_code")
		return crterrors.AbortWithError(ctx, http.StatusBadRequest, err, "error reading request body")
	}

	if phone.CountryCode == "" || phone.PhoneNumber == "" {
		log.Error(ctx, nil, "request body does not contain required fields phone_number and country_code")
		return ctx.NoContent(http.StatusBadRequest)
	}

	countryCode, err := strconv.Atoi(phone.CountryCode)
	if err != nil {
		log.Errorf(ctx, err, "invalid country_code value")
		return crterrors.AbortWithError(ctx, http.StatusBadRequest, err, "invalid country_code")
	}

	regionCode := phonenumbers.GetRegionCodeForCountryCode(countryCode)
	number, err := phonenumbers.Parse(phone.PhoneNumber, regionCode)
	if err != nil {
		log.Errorf(ctx, err, "invalid phone number")
		return crterrors.AbortWithError(ctx, http.StatusBadRequest, err, "invalid phone number provided")
	}

	e164Number := phonenumbers.Format(number, phonenumbers.E164)
	err = s.app.VerificationService().InitVerification(ctx, username, e164Number, strconv.Itoa(countryCode))
	if err != nil {
		log.Errorf(ctx, err, "Verification for %s could not be sent", username)
		e := &crterrors.Error{}
		switch {
		case errors.As(err, &e):
			return crterrors.AbortWithError(ctx, int(e.Code), err, e.Message)
		default:
			return crterrors.AbortWithError(ctx, http.StatusInternalServerError, err, "error while initiating verification")
		}
	}

	log.Infof(ctx, "phone verification has been sent for username %s", username)
	return ctx.NoContent(http.StatusNoContent)
}

// GetHandler returns the Signup resource
func (s *Signup) GetHandler(ctx echo.Context) error {

	// Get the UserSignup resource from the service by the username
	username := context.GetString(ctx, context.UsernameKey)
	signupResource, err := s.app.SignupService().GetSignup(ctx, username, true)
	if err != nil {
		log.Error(ctx, err, "error getting UserSignup resource")
		e := &apierrors.StatusError{}
		if errors.As(err, &e) {
			return crterrors.AbortWithError(ctx, int(e.Status().Code), err, "error getting UserSignup resource")
		}
		return crterrors.AbortWithError(ctx, http.StatusInternalServerError, err, "error getting UserSignup resource")
	}
	if signupResource == nil {
		log.Infof(ctx, "UserSignup resource for username '%s' resource not found", username)
		return ctx.NoContent(http.StatusNotFound)
	}
	return ctx.JSON(http.StatusOK, signupResource)
}

// VerifyPhoneCodeHandler validates the phone verification code passed in by the user
func (s *Signup) VerifyPhoneCodeHandler(ctx echo.Context) error {
	log.Info(ctx, "Verifying phone code")
	code := ctx.Param("code")
	if code == "" {
		log.Error(ctx, nil, "no phone code provided in the request")
		return ctx.NoContent(http.StatusBadRequest)
	}

	username := context.GetString(ctx, context.UsernameKey)

	err := s.app.VerificationService().VerifyPhoneCode(ctx, username, code)
	if err != nil {
		e := &crterrors.Error{}
		switch {
		case errors.As(err, &e):
			return crterrors.AbortWithError(ctx, int(e.Code), err, "error while verifying phone code")
		default:
			return crterrors.AbortWithError(ctx, http.StatusInternalServerError, err, "unexpected error while verifying phone code")
		}
	}
	log.Info(ctx, "Verified phone code")
	return ctx.NoContent(http.StatusOK)
}

// VerifyActivationCodeHandler validates the activation code passed in by the user as a form value
func (s *Signup) VerifyActivationCodeHandler(ctx echo.Context) error {
	body := map[string]interface{}{}
	if err := ctx.Bind(&body); err != nil {
		log.Error(ctx, nil, "no activation code provided in the request")
		return ctx.NoContent(http.StatusBadRequest)
	}
	code, ok := body["code"].(string)
	if !ok || code == "" {
		log.Error(ctx, nil, "no activation code provided in the request")
		return ctx.NoContent(http.StatusBadRequest)
	}

	username := context.GetString(ctx, context.UsernameKey)

	err := s.app.VerificationService().VerifyActivationCode(ctx, username, code)
	if err != nil {
		log.Error(ctx, err, "error validating activation code")
		e := &crterrors.Error{}
		switch {
		case errors.As(err, &e):
			return crterrors.AbortWithError(ctx, int(e.Code), err, "error while verifying activation code")
		default:
			return crterrors.AbortWithError(ctx, http.StatusInternalServerError, err, "unexpected error while verifying activation code")
		}
	}
	return ctx.NoContent(http.StatusOK)
}
