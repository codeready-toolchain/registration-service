package service

import (
	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/namespace"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/gin-gonic/gin"
)

type SignupService interface {
	Signup(ctx *gin.Context) (*toolchainv1alpha1.UserSignup, error)
	GetSignup(userID, username string) (*signup.Signup, error)
	GetUserSignup(userID, username string) (*toolchainv1alpha1.UserSignup, error)
	UpdateUserSignup(userSignup *toolchainv1alpha1.UserSignup) (*toolchainv1alpha1.UserSignup, error)
	PhoneNumberAlreadyInUse(userID, username, phoneNumberOrHash string) error
}

type SocialEventService interface {
	GetEvent(code string) (*toolchainv1alpha1.SocialEvent, error)
}

type VerificationService interface {
	InitVerification(ctx *gin.Context, userID, username, e164PhoneNumber string) error
	VerifyPhoneCode(ctx *gin.Context, userID, username, code string) error
	VerifyActivationCode(ctx *gin.Context, userID, username, code string) error
}

type MemberClusterService interface {
	GetClusterAccess(ctx *gin.Context, userID, username string) (*namespace.ClusterAccess, error)
}

type Services interface {
	SignupService() SignupService
	VerificationService() VerificationService
	MemberClusterService() MemberClusterService
}
