package service

import (
	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/labstack/echo/v4"
)

type SignupService interface {
	Signup(ctx echo.Context) (*toolchainv1alpha1.UserSignup, error)
	GetSignup(ctx echo.Context, username string, checkUserSignupCompleted bool) (*signup.Signup, error)
}

type VerificationService interface {
	InitVerification(ctx echo.Context, username, e164PhoneNumber, countryCode string) error
	VerifyPhoneCode(ctx echo.Context, username, code string) error
	VerifyActivationCode(ctx echo.Context, username, code string) error
}

type Services interface {
	SignupService() SignupService
	VerificationService() VerificationService
}
