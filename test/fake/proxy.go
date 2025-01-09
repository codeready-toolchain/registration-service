package fake

import (
	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/gin-gonic/gin"
)

// This whole service abstraction is such a huge pain. We have to get rid of it!!!

func NewSignupService(signups ...*signup.Signup) *SignupService {
	sc := newFakeSignupService()
	for _, signup := range signups {
		sc.addSignup(signup.Name, signup)
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
	MockGetSignup func(username string) (*signup.Signup, error)
	userSignups   map[string]*signup.Signup
}

func (m *SignupService) DefaultMockGetSignup() func(username string) (*signup.Signup, error) {
	return func(username string) (userSignup *signup.Signup, e error) {
		return m.userSignups[username], nil
	}
}

func (m *SignupService) GetSignup(_ *gin.Context, username string, _ bool) (*signup.Signup, error) {
	return m.MockGetSignup(username)
}

func (m *SignupService) Signup(_ *gin.Context) (*toolchainv1alpha1.UserSignup, error) {
	return nil, nil
}
func (m *SignupService) UpdateUserSignup(_ *toolchainv1alpha1.UserSignup) (*toolchainv1alpha1.UserSignup, error) {
	return nil, nil
}
func (m *SignupService) PhoneNumberAlreadyInUse(_, _ string) error {
	return nil
}
