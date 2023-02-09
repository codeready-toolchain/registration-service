package fake

import (
	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/informers"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/access"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/gin-gonic/gin"
)

// This whole service abstraction is such a huge pain. We have to get rid of it!!!

type ProxyFakeApp struct {
	Accesses                 map[string]*access.ClusterAccess
	Err                      error
	SignupServiceMock        service.SignupService
	MemberClusterServiceMock service.MemberClusterService
}

func (a *ProxyFakeApp) InformerService() service.InformerService {
	panic("InformerService shouldn't be called")
}

func (a *ProxyFakeApp) SignupService() service.SignupService {
	if a.SignupServiceMock != nil {
		return a.SignupServiceMock
	}
	return NewSignupService()
}

func (a *ProxyFakeApp) VerificationService() service.VerificationService {
	panic("VerificationService shouldn't be called")
}

func (a *ProxyFakeApp) MemberClusterService() service.MemberClusterService {
	if a.MemberClusterServiceMock != nil {
		return a.MemberClusterServiceMock
	}
	return &fakeClusterService{a}
}

type fakeClusterService struct {
	fakeApp *ProxyFakeApp
}

func (f *fakeClusterService) GetClusterAccess(userID, _, _ string) (*access.ClusterAccess, error) {
	return f.fakeApp.Accesses[userID], f.fakeApp.Err
}

type SignupDef func() (string, *signup.Signup)

func Signup(identifier string, userSignup *signup.Signup) SignupDef {
	return func() (string, *signup.Signup) {
		return identifier, userSignup
	}
}

func NewSignupService(signupDefs ...SignupDef) *SignupService {
	sc := newFakeSignupService()
	for _, signupDef := range signupDefs {
		identifier, signup := signupDef()
		sc.addSignup(identifier, signup)
	}
	return sc
}

func newFakeSignupService() *SignupService {
	f := &SignupService{}
	f.MockGetSignup = f.DefaultMockGetSignup()
	return f
}

func (m *SignupService) addSignup(identifier string, userSignup *signup.Signup) *SignupService {
	if m.userSignups == nil {
		m.userSignups = make(map[string]*signup.Signup)
	}
	m.userSignups[identifier] = userSignup
	return m
}

type SignupService struct {
	MockGetSignup func(userID, username string) (*signup.Signup, error)
	userSignups   map[string]*signup.Signup
}

func (m *SignupService) DefaultMockGetSignup() func(userID, username string) (*signup.Signup, error) {
	return func(userID, username string) (userSignup *signup.Signup, e error) {
		us := m.userSignups[userID]
		if us != nil {
			return us, nil
		}
		for _, v := range m.userSignups {
			if v.Username == username {
				return v, nil
			}
		}
		return nil, nil
	}
}

func (m *SignupService) GetSignup(userID, username string) (*signup.Signup, error) {
	return m.MockGetSignup(userID, username)
}

func (m *SignupService) GetSignupFromInformer(userID, username string) (*signup.Signup, error) {
	return m.MockGetSignup(userID, username)
}

func (m *SignupService) Signup(_ *gin.Context) (*toolchainv1alpha1.UserSignup, error) {
	return nil, nil
}
func (m *SignupService) GetUserSignupFromIdentifier(_, _ string) (*toolchainv1alpha1.UserSignup, error) {
	return nil, nil
}
func (m *SignupService) UpdateUserSignup(_ *toolchainv1alpha1.UserSignup) (*toolchainv1alpha1.UserSignup, error) {
	return nil, nil
}
func (m *SignupService) PhoneNumberAlreadyInUse(_, _, _ string) error {
	return nil
}

type MemberClusterServiceContext struct {
	Client kubeclient.CRTClient
	Svcs   service.Services
}

func (sc MemberClusterServiceContext) CRTClient() kubeclient.CRTClient {
	return sc.Client
}

func (sc MemberClusterServiceContext) Informer() informers.Informer {
	panic("shouldn't need informer in mock member cluster service")
}

func (sc MemberClusterServiceContext) Services() service.Services {
	return sc.Svcs
}
