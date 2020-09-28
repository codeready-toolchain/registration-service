package controller

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/registration-service/pkg/verification"

	"github.com/gin-gonic/gin"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// Signup implements the signup endpoint, which is invoked for new user registrations.
type Signup struct {
	config              *configuration.Config
	signupService       signup.Service
	verificationService verification.Service
}

type Phone struct {
	CountryCode string `form:"country_code" json:"country_code" binding:"required"`
	PhoneNumber string `form:"phone_number" json:"phone_number" binding:"required"`
}

// NewSignup returns a new Signup instance.
func NewSignup(config *configuration.Config, signupService signup.Service, verificationService verification.Service) *Signup {
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

// UpdateVerificationHandler starts the verification process and updates a usersignup resource. The
// ctx should contain a JSON body in the request with a country_code and a phone_number string
func (s *Signup) UpdateVerificationHandler(ctx *gin.Context) {
	userID := ctx.GetString(context.SubKey)
	signup, err := s.signupService.GetUserSignup(userID)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Error(ctx, err, "usersignup not found")
			errors.AbortWithError(ctx, http.StatusNotFound, err, "usersignup not found")
			return
		}
		log.Error(ctx, err, "error retrieving usersignup")
		errors.AbortWithError(ctx, http.StatusInternalServerError, err, fmt.Sprintf("error retrieving usersignup: %s", userID))
		return
	}

	// check that verification is required before proceeding
	if signup.Spec.VerificationRequired == false {
		log.Errorf(ctx, errors.NewForbiddenError("forbidden request", "verification code will not be sent"), "phone verification not required for usersignup: %s", userID)
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}

	// Read the Body content
	var phone Phone
	if err := ctx.BindJSON(&phone); err != nil {
		log.Errorf(ctx, err, "request body does not contain required fields phone_number and country_code")
		errors.AbortWithError(ctx, http.StatusBadRequest, err, "error reading request body")
		return
	}

	// normalize phone number
	r := strings.NewReplacer("(", "",
		")", "",
		" ", "",
		"-", "")
	countryCode := r.Replace(phone.CountryCode)
	phoneNumber := r.Replace(phone.PhoneNumber)
	countryValid, _ := regexp.MatchString(`^\+?[0-9]+$`, countryCode)
	phoneValid, _ := regexp.MatchString(`^[0-9]+$`, phoneNumber)

	// if phone number contains odd characters, return error
	if !countryValid || !phoneValid {
		log.Errorf(ctx, errors.NewBadRequest("bad request", "invalid request"), "phone number entered contains invalid characters")
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}
	phone.PhoneNumber = phoneNumber
	phone.CountryCode = countryCode

	err = s.signupService.PhoneNumberAlreadyInUse(userID, phone.CountryCode, phone.PhoneNumber)
	if err != nil {
		if apierrors.IsForbidden(err) {
			log.Errorf(ctx, err, "phone number already in use, cannot register using phone number: %s", phone.CountryCode+phone.PhoneNumber)
			errors.AbortWithError(ctx, http.StatusForbidden, err, fmt.Sprintf("phone number already in use, cannot register using phone number: %s", phone.CountryCode+phone.PhoneNumber))
			return
		}
		log.Error(ctx, err, "error while looking up users by phone number")
		errors.AbortWithError(ctx, http.StatusInternalServerError, err, "could not lookup users by phone number")
		return
	}

	signup, err = s.verificationService.InitVerification(ctx, signup, phone.CountryCode, phone.PhoneNumber)
	_, err2 := s.signupService.UpdateUserSignup(signup)
	if err2 != nil {
		log.Error(ctx, err2, "error while updating UserSignup resource")
		errors.AbortWithError(ctx, http.StatusInternalServerError, err2, "error while updating UserSignup resource")

		if err != nil {
			log.Error(ctx, err, "error initiating user verification")
		}
		return
	}

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

	signup, err := s.signupService.GetUserSignup(userID)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Error(ctx, err, "usersignup not found")
			errors.AbortWithError(ctx, http.StatusNotFound, err, "usersignup not found")
		}
		log.Error(ctx, err, "error retrieving usersignup")
		errors.AbortWithError(ctx, http.StatusInternalServerError, err, fmt.Sprintf("error retrieving usersignup: %s", userID))
		return
	}

	// The VerifyCode() call here MAY make changes to the specified signupResource
	signup, err = s.verificationService.VerifyCode(signup, code)

	// Regardless of whether the VerifyCode() call returns an error or not, we need to update the UserSignup instance
	// as its state can be updated even in the case of an error.  This may result in the slight possibility that any
	// errors returned by VerifyCode() are suppressed, as error handling for the UserSignup update is given precedence.
	_, err2 := s.signupService.UpdateUserSignup(signup)
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
			errors.AbortWithError(ctx, http.StatusInternalServerError, err, "unexpected error while verifying code")
		case *errors.Error:
			errors.AbortWithError(ctx, int(t.Code), err, "error while verifying code")
		}
		return
	}
	ctx.Status(http.StatusOK)
}
