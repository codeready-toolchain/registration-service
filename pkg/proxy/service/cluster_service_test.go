package service_test

import (
	"testing"
	"time"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	appservice "github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/service"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/registration-service/test"
	commoncluster "github.com/codeready-toolchain/toolchain-common/pkg/cluster"
	commonconfig "github.com/codeready-toolchain/toolchain-common/pkg/configuration"
	testsupport "github.com/codeready-toolchain/toolchain-common/pkg/test"
	testconfig "github.com/codeready-toolchain/toolchain-common/pkg/test/config"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	TestNamespace = "test-namespace-123"
)

type TestClusterServiceSuite struct {
	test.UnitTestSuite
}

func TestRunClusterServiceSuite(t *testing.T) {
	suite.Run(t, &TestClusterServiceSuite{test.UnitTestSuite{}})
}

func (s *TestClusterServiceSuite) ServiceConfiguration() {
	testsupport.SetEnvVarAndRestore(s.T(), commonconfig.WatchNamespaceEnvVar, TestNamespace)

	s.OverrideApplicationDefault(
		testconfig.RegistrationService().
			Verification().Enabled(false))
}

func (s *TestClusterServiceSuite) TestGetNamespace() {
	// given

	sc := newFakeSignupService().addSignup("123-noise", &signup.Signup{
		CompliantUsername: "noise1",
		Username:          "noise1",
		Status: signup.Status{
			Ready: true,
		},
	}).addSignup("456-not-ready", &signup.Signup{
		CompliantUsername: "john",
		Username:          "john",
		Status: signup.Status{
			Ready: false,
		},
	}).addSignup("789-ready", &signup.Signup{
		CompliantUsername: "smith",
		Username:          "smith",
		Status: signup.Status{
			Ready: true,
		},
	})
	s.Application.MockSignupService(sc)

	keys := make(map[string]interface{})
	keys[context.SubKey] = "unknown_id"
	ctx := &gin.Context{Keys: keys}

	// Initiate toolchain cluster service in common
	cl := testsupport.NewFakeClient(s.T())
	commoncluster.NewToolchainClusterService(cl, log.Log, TestNamespace, 5*time.Second)

	svc := service.NewToolchainClusterService(serviceContext{
		cl:  s,
		svc: s.Application,
	})

	s.Run("user is not provisioned yet", func() {
		// when
		_, err := svc.GetNamespace(ctx, "unknown_id")

		// then
		require.EqualError(s.T(), err, "user is not (yet) provisioned")
	})

	s.Run("no member clusters", func() {
		// when
		_, err := svc.GetNamespace(ctx, "789-ready")

		// then
		require.EqualError(s.T(), err, "no member clusters found")
	})

	s.Run("usersignup service returns error", func() {
		// TODO
	})

}

type serviceContext struct {
	cl  kubeclient.CRTClient
	svc appservice.Services
}

func (sc serviceContext) CRTClient() kubeclient.CRTClient {
	return sc.cl
}

func (sc serviceContext) Services() appservice.Services {
	return sc.svc
}

func newFakeSignupService() *fakeSignupService {
	f := &fakeSignupService{}
	f.mockGetSignup = f.defaultMockGetSignup()
	return f
}

func (m *fakeSignupService) addSignup(userID string, userSignup *signup.Signup) *fakeSignupService {
	if m.userSignups == nil {
		m.userSignups = make(map[string]*signup.Signup)
	}
	m.userSignups[userID] = userSignup
	return m
}

type fakeSignupService struct {
	mockGetSignup func(userID string) (*signup.Signup, error)
	userSignups   map[string]*signup.Signup
}

func (m *fakeSignupService) defaultMockGetSignup() func(userID string) (*signup.Signup, error) {
	return func(userID string) (userSignup *signup.Signup, e error) {
		return m.userSignups[userID], nil
	}
}

func (m *fakeSignupService) GetSignup(userID string) (*signup.Signup, error) {
	return m.mockGetSignup(userID)
}

func (m *fakeSignupService) Signup(_ *gin.Context) (*toolchainv1alpha1.UserSignup, error) {
	return nil, nil
}
func (m *fakeSignupService) GetUserSignup(_ string) (*toolchainv1alpha1.UserSignup, error) {
	return nil, nil
}
func (m *fakeSignupService) UpdateUserSignup(_ *toolchainv1alpha1.UserSignup) (*toolchainv1alpha1.UserSignup, error) {
	return nil, nil
}
func (m *fakeSignupService) PhoneNumberAlreadyInUse(_, _ string) error {
	return nil
}
