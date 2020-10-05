package controller

import (
	"net/http"
	"strconv"

	"github.com/codeready-toolchain/registration-service/pkg/application"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/codeready-toolchain/registration-service/pkg/log"

	"github.com/gin-gonic/gin"
	"github.com/nyaruka/phonenumbers"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// Signup implements the signup endpoint, which is invoked for new user registrations.
type Signup struct {
	app    application.Application
	config *configuration.Config
}

type Phone struct {
	CountryCode string `form:"country_code" json:"country_code" binding:"required"`
	PhoneNumber string `form:"phone_number" json:"phone_number" binding:"required"`
}

// NewSignup returns a new Signup instance.
func NewSignup(app application.Application, config *configuration.Config) *Signup {
	return &Signup{
		app:    app,
		config: config,
	}
}

// PostHandler creates a Signup resource
func (s *Signup) PostHandler(ctx *gin.Context) {
	userSignup, err := s.app.SignupService().CreateUserSignup(ctx)
	if err != nil {
		log.Error(ctx, err, "error creating UserSignup resource")
		errors.AbortWithError(ctx, http.StatusInternalServerError, err, "error creating UserSignup resource")
		return
	}

	log.Infof(ctx, "UserSignup %s created", userSignup.Name)
	ctx.Status(http.StatusAccepted)
	ctx.Writer.WriteHeaderNow()
}

// InitVerificationHandler starts the verification process and updates a usersignup resource. The
// ctx should contain a JSON body in the request with a country_code and a phone_number string
func (s *Signup) InitVerificationHandler(ctx *gin.Context) {
	userID := ctx.GetString(context.SubKey)

	// Read the Body content
	var phone Phone
	if err := ctx.BindJSON(&phone); err != nil {
		log.Errorf(ctx, err, "request body does not contain required fields phone_number and country_code")
		errors.AbortWithError(ctx, http.StatusBadRequest, err, "error reading request body")
		return
	}

	countryCode, err := strconv.Atoi(phone.CountryCode)
	if err != nil {
		log.Errorf(ctx, err, "invalid country_code value")
		errors.AbortWithError(ctx, http.StatusBadRequest, err, "invalid country_code")
	}

	regionCode := phonenumbers.GetRegionCodeForCountryCode(countryCode)
	number, err := phonenumbers.Parse(phone.PhoneNumber, regionCode)
	if err != nil {
		log.Errorf(ctx, err, "invalid phone number")
		errors.AbortWithError(ctx, http.StatusBadRequest, err, "invalid phone number provided")
	}

	e164Number := phonenumbers.Format(number, phonenumbers.E164)

	err = s.app.VerificationService().InitVerification(ctx, userID, e164Number)
	if err != nil {
		log.Errorf(ctx, nil, "Verification for %s could not be sent", userID)
		switch t := err.(type) {
		default:
			errors.AbortWithError(ctx, http.StatusInternalServerError, err, "error while initiating verification")
		case *apierrors.StatusError:
			errors.AbortWithError(ctx, int(t.ErrStatus.Code), err, t.ErrStatus.Message)
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
	signupResource, err := s.app.SignupService().GetSignup(userID)
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

	err := s.app.VerificationService().VerifyCode(ctx, userID, code)
	if err != nil {
		log.Error(ctx, err, "error validating user verification code")
		switch t := err.(type) {
		default:
			errors.AbortWithError(ctx, http.StatusInternalServerError, err, "unexpected error while verifying code")
		case *errors.Error:
			errors.AbortWithError(ctx, int(t.Code), err, "error while verifying code")
		}
		return
	}
	ctx.Status(http.StatusOK)
}
