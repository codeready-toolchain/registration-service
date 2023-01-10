package service_test

import (
	"errors"
	"fmt"
	"net/url"
	"testing"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	appservice "github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/informers"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/access"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/service"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/registration-service/test"
	"github.com/codeready-toolchain/registration-service/test/fake"

	commoncluster "github.com/codeready-toolchain/toolchain-common/pkg/cluster"
	commontest "github.com/codeready-toolchain/toolchain-common/pkg/test"
	murtest "github.com/codeready-toolchain/toolchain-common/pkg/test/masteruserrecord"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"k8s.io/client-go/rest"
)

type TestClusterServiceSuite struct {
	test.UnitTestSuite
}

func TestRunClusterServiceSuite(t *testing.T) {
	suite.Run(t, &TestClusterServiceSuite{test.UnitTestSuite{}})
}

func (s *TestClusterServiceSuite) TestGetClusterAccess() {
	// given

	is := newFakeInformerService(fake.Signup("123-noise", &signup.Signup{
		CompliantUsername: "noise1",
		Username:          "noise1",
		Status: signup.Status{
			Ready: true,
		},
	}), fake.Signup("456-not-ready", &signup.Signup{
		CompliantUsername: "john",
		Username:          "john@",
		Status: signup.Status{
			Ready: false,
		},
	}), fake.Signup("789-ready", &signup.Signup{
		APIEndpoint:       "https://api.endpoint.member-2.com:6443",
		ClusterName:       "member-2",
		CompliantUsername: "smith2",
		Username:          "smith@",
		Status: signup.Status{
			Ready: true,
		},
	}), fake.Signup("012-ready-unknown-cluster", &signup.Signup{
		APIEndpoint:       "https://api.endpoint.unknown.com:6443",
		ClusterName:       "unknown",
		CompliantUsername: "doe",
		Username:          "doe@",
		Status: signup.Status{
			Ready: true,
		},
	}))
	s.Application.MockInformerService(is)

	svc := service.NewMemberClusterService(
		serviceContext{
			cl:  s,
			svc: s.Application,
		},
	)

	s.Run("unable to get signup", func() {
		s.Run("informer service returns error", func() {
			is.mockGetSignup = func(userID, username string) (*signup.Signup, error) {
				return nil, errors.New("oopsi woopsi")
			}

			// when
			_, err := svc.GetClusterAccess("789-ready", "")

			// then
			require.EqualError(s.T(), err, "oopsi woopsi")
		})

		is.mockGetSignup = is.defaultMockGetSignup() // restore the default signup service, so it doesn't return an error anymore

		s.Run("user is not found", func() {
			// when
			_, err := svc.GetClusterAccess("unknown_id", "")

			// then
			require.EqualError(s.T(), err, "user is not provisioned (yet)")
		})

		s.Run("user is not found", func() {
			// when
			_, err := svc.GetClusterAccess("", "unknown_username")

			// then
			require.EqualError(s.T(), err, "user is not provisioned (yet)")
		})

		s.Run("user is not provisioned yet", func() {
			// when
			_, err := svc.GetClusterAccess("456-not-ready", "")

			// then
			require.EqualError(s.T(), err, "user is not provisioned (yet)")
		})
	})

	s.Run("no member cluster found", func() {
		s.Run("no member clusters", func() {
			svc := service.NewMemberClusterService(
				serviceContext{
					cl:  s,
					svc: s.Application,
				},
				func(si *service.ServiceImpl) {
					si.GetMembersFunc = func(conditions ...commoncluster.Condition) []*commoncluster.CachedToolchainCluster {
						return []*commoncluster.CachedToolchainCluster{}
					}
				},
			)

			// when
			_, err := svc.GetClusterAccess("789-ready", "")

			// then
			require.EqualError(s.T(), err, "no member clusters found")
		})

		s.Run("no member cluster with the given URL", func() {
			svc := service.NewMemberClusterService(
				serviceContext{
					cl:  s,
					svc: s.Application,
				},
				func(si *service.ServiceImpl) {
					si.GetMembersFunc = func(conditions ...commoncluster.Condition) []*commoncluster.CachedToolchainCluster {
						return s.memberClusters()
					}
				},
			)

			// when
			_, err := svc.GetClusterAccess("012-ready-unknown-cluster", "")

			// then
			require.EqualError(s.T(), err, "no member cluster found for the user")
		})
	})

	s.Run("member found", func() {
		memberClient := commontest.NewFakeClient(s.T())

		svc := service.NewMemberClusterService(
			serviceContext{
				cl:  s,
				svc: s.Application,
			},
			func(si *service.ServiceImpl) {
				si.GetMembersFunc = func(conditions ...commoncluster.Condition) []*commoncluster.CachedToolchainCluster {
					return []*commoncluster.CachedToolchainCluster{
						{
							Config: &commoncluster.Config{
								Name:        "member-1",
								Type:        commoncluster.Member,
								APIEndpoint: "https://api.endpoint.member-1.com:6443",
								RestConfig:  &rest.Config{},
							},
						},
						{
							Config: &commoncluster.Config{
								Name:              "member-2",
								APIEndpoint:       "https://api.endpoint.member-2.com:6443",
								Type:              commoncluster.Member,
								OperatorNamespace: "member-operator",
								RestConfig: &rest.Config{
									BearerToken: "abc123",
								},
							},
							Client: memberClient,
						},
						{
							Config: &commoncluster.Config{
								Name:        "member-3",
								APIEndpoint: "https://api.endpoint.member-3.com:6443",
								Type:        commoncluster.Member,
								RestConfig:  &rest.Config{},
							},
						},
					}
				}
			},
		)

		s.Run("verify cluster access", func() {
			memberClient.MockGet = nil
			expectedToken := "abc123" // should match member 2 bearer token

			// when
			ca, err := svc.GetClusterAccess("789-ready", "")

			// then
			require.NoError(s.T(), err)
			require.NotNil(s.T(), ca)
			expectedURL, err := url.Parse("https://api.endpoint.member-2.com:6443")
			require.NoError(s.T(), err)

			s.assertClusterAccess(access.NewClusterAccess(*expectedURL, expectedToken, ""), ca)

			s.Run("cluster access correct when username provided", func() {
				// when
				ca, err := svc.GetClusterAccess("", "smith@")

				// then
				require.NoError(s.T(), err)
				require.NotNil(s.T(), ca)
				expectedURL, err := url.Parse("https://api.endpoint.member-2.com:6443")
				require.NoError(s.T(), err)
				s.assertClusterAccess(access.NewClusterAccess(*expectedURL, expectedToken, "smith"), ca)
			})
		})
	})
}

func (s *TestClusterServiceSuite) assertClusterAccess(expected, actual *access.ClusterAccess) {
	require.NotNil(s.T(), expected)
	require.NotNil(s.T(), actual)
	assert.Equal(s.T(), expected.APIURL(), actual.APIURL())
	assert.Equal(s.T(), expected.ImpersonatorToken(), actual.ImpersonatorToken())
}

func (s *TestClusterServiceSuite) memberClusters() []*commoncluster.CachedToolchainCluster {
	cls := make([]*commoncluster.CachedToolchainCluster, 0, 3)
	for i := 0; i < 3; i++ {
		clusterName := fmt.Sprintf("member-%d", i)

		cls = append(cls, &commoncluster.CachedToolchainCluster{
			Config: &commoncluster.Config{
				Name:              clusterName,
				APIEndpoint:       fmt.Sprintf("https://api.endpoint.%s.com:6443", clusterName),
				Type:              commoncluster.Member,
				OperatorNamespace: "member-operator",
			},
			Client: nil,
		})
	}
	return cls
}

type serviceContext struct {
	cl       kubeclient.CRTClient
	informer informers.Informer
	svc      appservice.Services
}

func (sc serviceContext) CRTClient() kubeclient.CRTClient {
	return sc.cl
}

func (sc serviceContext) Informer() informers.Informer {
	return sc.informer
}

func (sc serviceContext) Services() appservice.Services {
	return sc.svc
}

func newFakeInformerService(signupDefs ...fake.SignupDef) *fakeInformerService {
	f := &fakeInformerService{}
	f.mockGetSignup = f.defaultMockGetSignup()
	for _, signupDef := range signupDefs {
		identifier, signup := signupDef()
		f.addSignup(identifier, signup)
	}
	return f
}

type fakeInformerService struct {
	mockGetSignup func(userID, username string) (*signup.Signup, error)
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
	return m.mockGetSignup(userID, username)
}

func (m *fakeInformerService) GetUserSignupFromIdentifier(userID, username string) (*toolchainv1alpha1.UserSignup, error) {
	panic("should not be called, tested separately")
}

func (m *fakeInformerService) defaultMockGetSignup() func(userID, username string) (*signup.Signup, error) {
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

func WithCluster(clusterName string) murtest.MurModifier {
	return func(mur *toolchainv1alpha1.MasterUserRecord) error {
		if mur.Status.UserAccounts == nil {
			mur.Status.UserAccounts = []toolchainv1alpha1.UserAccountStatusEmbedded{}
		}
		mur.Status.UserAccounts = append(mur.Status.UserAccounts, toolchainv1alpha1.UserAccountStatusEmbedded{
			Cluster: toolchainv1alpha1.Cluster{
				Name: clusterName,
			},
		})
		return nil
	}
}
