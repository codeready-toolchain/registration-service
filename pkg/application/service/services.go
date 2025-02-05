package service

import (
	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/gin-gonic/gin"
)

type SignupService interface {
	Signup(ctx *gin.Context) (*toolchainv1alpha1.UserSignup, error)
	GetSignup(ctx *gin.Context, username string, checkUserSignupCompleted bool) (*signup.Signup, error)
}

type VerificationService interface {
	InitVerification(ctx *gin.Context, username, e164PhoneNumber, countryCode string) error
	VerifyPhoneCode(ctx *gin.Context, username, code string) error
	VerifyActivationCode(ctx *gin.Context, username, code string) error
}

type Services interface {
	SignupService() SignupService
	VerificationService() VerificationService
}
