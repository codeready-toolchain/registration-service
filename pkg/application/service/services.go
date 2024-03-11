package service

import (
	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/access"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/labels"
)

type InformerService interface {
	GetMasterUserRecord(name string) (*toolchainv1alpha1.MasterUserRecord, error)
	GetUserTier(name string) (*toolchainv1alpha1.UserTier, error)
	GetSpace(name string) (*toolchainv1alpha1.Space, error)
	GetToolchainStatus() (*toolchainv1alpha1.ToolchainStatus, error)
	GetUserSignup(name string) (*toolchainv1alpha1.UserSignup, error)
	ListSpaceBindings(reqs ...labels.Requirement) ([]toolchainv1alpha1.SpaceBinding, error)
	GetProxyPluginConfig(name string) (*toolchainv1alpha1.ProxyPlugin, error)
	GetNSTemplateTier(name string) (*toolchainv1alpha1.NSTemplateTier, error)
}

type SignupService interface {
	Signup(ctx *gin.Context) (*toolchainv1alpha1.UserSignup, error)
	GetSignup(ctx *gin.Context, userID, username string) (*signup.Signup, error)
	GetSignupFromInformer(ctx *gin.Context, userID, username string, checkUserSignupCompleted bool) (*signup.Signup, error)
	GetUserSignupFromIdentifier(userID, username string) (*toolchainv1alpha1.UserSignup, error)
	UpdateUserSignup(userSignup *toolchainv1alpha1.UserSignup) (*toolchainv1alpha1.UserSignup, error)
	PhoneNumberAlreadyInUse(userID, username, phoneNumberOrHash string) error
}

type SocialEventService interface {
	GetEvent(code string) (*toolchainv1alpha1.SocialEvent, error)
}

type VerificationService interface {
	InitVerification(ctx *gin.Context, userID, username, e164PhoneNumber, countryCode string) error
	VerifyPhoneCode(ctx *gin.Context, userID, username, code string) error
	VerifyActivationCode(ctx *gin.Context, userID, username, code string) error
}

type MemberClusterService interface {
	GetClusterAccess(userID, username, workspace, proxyPluginName string) (*access.ClusterAccess, error)
}

type Services interface {
	InformerService() InformerService
	SignupService() SignupService
	VerificationService() VerificationService
	MemberClusterService() MemberClusterService
}
