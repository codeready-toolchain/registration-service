package service

import (
	"github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/gin-gonic/gin"
)

type SignupService interface {
	Signup(ctx *gin.Context) (*v1alpha1.UserSignup, error)
	GetSignup(userID string) (*signup.Signup, error)
	GetUserSignup(userID string) (*v1alpha1.UserSignup, error)
	UpdateUserSignup(userSignup *v1alpha1.UserSignup) (*v1alpha1.UserSignup, error)
	PhoneNumberAlreadyInUse(userID, e164PhoneNumber, email string) error
}

type VerificationService interface {
	InitVerification(ctx *gin.Context, userID string, e164PhoneNumber string) error
	VerifyCode(ctx *gin.Context, userID string, code string) error
}

type Services interface {
	SignupService() SignupService
	VerificationService() VerificationService
}
