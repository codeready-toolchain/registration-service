package fake

import (
	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	appservice "github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/informers"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
)

type InformerServiceContext struct {
	cl       kubeclient.CRTClient
	informer informers.Informer
	svc      appservice.Services
}

func (sc InformerServiceContext) CRTClient() kubeclient.CRTClient {
	return sc.cl
}

func (sc InformerServiceContext) Informer() informers.Informer {
	return sc.informer
}

func (sc InformerServiceContext) Services() appservice.Services {
	return sc.svc
}

func NewInformerService(signupDefs ...SignupDef) *fakeInformerService {
	f := &fakeInformerService{}
	f.MockGetSignup = f.DefaultMockGetSignup()
	for _, signupDef := range signupDefs {
		identifier, signup := signupDef()
		f.addSignup(identifier, signup)
	}
	return f
}

type fakeInformerService struct {
	MockGetSignup func(userID, username string) (*signup.Signup, error)
	signups       map[string]*signup.Signup
}

func (m *fakeInformerService) GetMasterUserRecord(name string) (*toolchainv1alpha1.MasterUserRecord, error) {
	panic("should not be called, tested separately")
}

func (m *fakeInformerService) GetToolchainStatus() (*toolchainv1alpha1.ToolchainStatus, error) {
	panic("should not be called, tested separately")
}

func (m *fakeInformerService) GetUserSignup(name string) (*toolchainv1alpha1.UserSignup, error) {
	panic("should not be called, tested separately")
}

func (m *fakeInformerService) GetSignup(userID, username string) (*signup.Signup, error) {
	return m.MockGetSignup(userID, username)
}

func (m *fakeInformerService) GetUserSignupFromIdentifier(userID, username string) (*toolchainv1alpha1.UserSignup, error) {
	panic("should not be called, tested separately")
}

func (m *fakeInformerService) DefaultMockGetSignup() func(userID, username string) (*signup.Signup, error) {
	return func(userID, username string) (userSignup *signup.Signup, e error) {
		signup := m.signups[userID]
		if signup != nil {
			return signup, nil
		}
		for _, v := range m.signups {
			if v.Username == username {
				return v, nil
			}
		}
		return nil, nil
	}
}

func (m *fakeInformerService) addSignup(identifier string, s *signup.Signup) *fakeInformerService {
	if m.signups == nil {
		m.signups = make(map[string]*signup.Signup)
	}
	m.signups[identifier] = s
	return m
}
