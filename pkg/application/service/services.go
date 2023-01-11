package service

import (
	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/access"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/gin-gonic/gin"
)

type InformerService interface {
	GetMasterUserRecord(name string) (*toolchainv1alpha1.MasterUserRecord, error)
	GetToolchainStatus() (*toolchainv1alpha1.ToolchainStatus, error)
	GetUserSignup(name string) (*toolchainv1alpha1.UserSignup, error)
	GetUserSignupFromIdentifier(userID, username string) (*toolchainv1alpha1.UserSignup, error)

	// GetSignup duplicates the logic of the 'GetSignup' function in the signup service, except it uses informers to get resources.
	// This function can be move to the signup service and replace the GetSignup function there once it is determined to be stable.
	GetSignup(userID, username string) (*signup.Signup, error)
}

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
	GetClusterAccess(userID, username string) (*access.ClusterAccess, error)
}

type Services interface {
	InformerService() InformerService
	SignupService() SignupService
	VerificationService() VerificationService
	MemberClusterService() MemberClusterService
}
