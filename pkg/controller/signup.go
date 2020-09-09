package controller

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

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

// UpdateVerificationHandler starts the verification process and updates a usersignup resource
func (s *Signup) UpdateVerificationHandler(ctx *gin.Context) {
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

	// check that verification is required before proceeding
	if signup.Spec.VerificationRequired == false {
		log.Errorf(ctx, nil, "phone verification not required for usersignup: %s", userID)
		ctx.AbortWithStatus(http.StatusBadRequest)
	}

	// Read the Body content
	defer ctx.Request.Body.Close()
	var bodyBytes []byte
	if ctx.Request.Body != nil {
		bodyBytes, err = ioutil.ReadAll(ctx.Request.Body)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, err)
			return
		}
	}

	m := make(map[string]string)
	err = json.Unmarshal(bodyBytes, &m)
	if err != nil {
		log.Errorf(ctx, nil, "Request body could not be read")
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	countryCode := m["country_code"]
	phoneNumber := m["phone_number"]
	err = s.signupService.CheckIfUserIsKnown(countryCode, phoneNumber)
	if err != nil {
		if apierrors.IsInternalError(err) {
			log.Error(ctx, err, "error while looking up users by phone number")
			errors.AbortWithError(ctx, http.StatusInternalServerError, err, "could not lookup users by phone number")
		}
		log.Errorf(ctx, err, "phone number already in use, cannot register using phone number: %s", countryCode+phoneNumber)
		errors.AbortWithError(ctx, http.StatusForbidden, err, fmt.Sprintf("phone number already in use, cannot register using phone number: %s", countryCode+phoneNumber))
	}

	signup, err = s.verificationService.InitVerification(ctx, signup, countryCode, phoneNumber)
	if err != nil {
		log.Errorf(ctx, nil, "Verification for %s could not be sent", userID)
		switch t := err.(type) {
		default:
			errors.AbortWithError(ctx, http.StatusInternalServerError, err, "error while verifying code")
		case *apierrors.StatusError:
			errors.AbortWithError(ctx, int(t.ErrStatus.Code), err, t.ErrStatus.Message)
		}
		return
	}

	log.Infof(ctx, "phone verification has been sent for userID %s", userID)
	ctx.Status(http.StatusOK)
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
			errors.AbortWithError(ctx, http.StatusInternalServerError, err, "error while verifying code")
		case *apierrors.StatusError:
			errors.AbortWithError(ctx, int(t.ErrStatus.Code), err, t.ErrStatus.Message)
		}
		return
	}
	ctx.Status(http.StatusOK)
}
