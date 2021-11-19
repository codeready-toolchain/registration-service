package service

import (
	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/namespace"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/gin-gonic/gin"
)

type SignupService interface {
	Activate(ctx *gin.Context, code string) (*toolchainv1alpha1.UserSignup, error)
	Signup(ctx *gin.Context) (*toolchainv1alpha1.UserSignup, error)
	GetSignup(userID string) (*signup.Signup, error)
	GetUserSignup(userID string) (*toolchainv1alpha1.UserSignup, error)
	UpdateUserSignup(userSignup *toolchainv1alpha1.UserSignup) (*toolchainv1alpha1.UserSignup, error)
	PhoneNumberAlreadyInUse(userID, phoneNumberOrHash string) error
}

type VerificationService interface {
	InitVerification(ctx *gin.Context, userID string, e164PhoneNumber string) error
	VerifyCode(ctx *gin.Context, userID string, code string) error
}

type MemberClusterService interface {
	GetNamespace(ctx *gin.Context, userID string) (*namespace.NamespaceAccess, error)
}

type Services interface {
	SignupService() SignupService
	VerificationService() VerificationService
	MemberClusterService() MemberClusterService
}
