package service_test

import (
	"errors"
	"fmt"
	"net/url"
	"testing"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	appservice "github.com/codeready-toolchain/registration-service/pkg/application/service"
	regservicecontext "github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/access"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/service"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/registration-service/test"
	commoncluster "github.com/codeready-toolchain/toolchain-common/pkg/cluster"
	commontest "github.com/codeready-toolchain/toolchain-common/pkg/test"

	"github.com/gin-gonic/gin"
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
		APIEndpoint:       "https://api.endpoint.member-2.com:6443",
		ClusterName:       "member-2",
		CompliantUsername: "smith",
		Username:          "smith",
		Status: signup.Status{
			Ready: true,
		},
	}).addSignup("012-ready-unknown-cluster", &signup.Signup{
		APIEndpoint:       "https://api.endpoint.unknown.com:6443",
		ClusterName:       "unknown",
		CompliantUsername: "doe",
		Username:          "doe",
		Status: signup.Status{
			Ready: true,
		},
	})
	s.Application.MockSignupService(sc)

	keys := make(map[string]interface{})
	keys[regservicecontext.SubKey] = "unknown_id"
	ctx := &gin.Context{Keys: keys}

	svc := service.NewMemberClusterService(
		serviceContext{
			cl:  s,
			svc: s.Application,
		},
	)

	s.Run("unable to get signup", func() {
		s.Run("signup service returns error", func() {
			sc.mockGetSignup = func(userID, username string) (*signup.Signup, error) {
				return nil, errors.New("oopsi woopsi")
			}

			// when
			_, err := svc.GetClusterAccess(ctx, "789-ready", "")

			// then
			require.EqualError(s.T(), err, "oopsi woopsi")
		})

		sc.mockGetSignup = sc.defaultMockGetSignup() // restore the default signup service, so it doesn't return an error anymore

		s.Run("user is not found", func() {
			// when
			_, err := svc.GetClusterAccess(ctx, "unknown_id", "")

			// then
			require.EqualError(s.T(), err, "user is not (yet) provisioned")
		})

		s.Run("user is not provisioned yet", func() {
			// when
			_, err := svc.GetClusterAccess(ctx, "456-not-ready", "")

			// then
			require.EqualError(s.T(), err, "user is not (yet) provisioned")
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
			_, err := svc.GetClusterAccess(ctx, "789-ready", "")

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
			_, err := svc.GetClusterAccess(ctx, "012-ready-unknown-cluster", "")

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
			ca, err := svc.GetClusterAccess(ctx, "789-ready", "")

			// then
			require.NoError(s.T(), err)
			require.NotNil(s.T(), ca)
			expectedURL, err := url.Parse("https://api.endpoint.member-2.com:6443")
			require.NoError(s.T(), err)

			s.assertClusterAccess(access.NewClusterAccess(*expectedURL, memberClient, expectedToken, ""), ca)

			s.Run("cluster access correct when username provided", func() {
				// when
				ca, err := svc.GetClusterAccess(ctx, "", "smith")

				// then
				require.NoError(s.T(), err)
				require.NotNil(s.T(), ca)
				expectedURL, err := url.Parse("https://api.endpoint.member-2.com:6443")
				require.NoError(s.T(), err)
				s.assertClusterAccess(access.NewClusterAccess(*expectedURL, memberClient, expectedToken, "smith"), ca)
			})
		})
	})
}

func (s *TestClusterServiceSuite) assertClusterAccess(expected, actual *access.ClusterAccess) {
	require.NotNil(s.T(), expected)
	require.NotNil(s.T(), actual)
	assert.Equal(s.T(), expected.APIURL(), actual.APIURL())
	assert.Equal(s.T(), expected.SAToken(), actual.SAToken())
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

func (m *fakeSignupService) addSignup(identifier string, userSignup *signup.Signup) *fakeSignupService {
	if m.userSignups == nil {
		m.userSignups = make(map[string]*signup.Signup)
	}
	m.userSignups[identifier] = userSignup
	return m
}

type fakeSignupService struct {
	mockGetSignup func(userID, username string) (*signup.Signup, error)
	userSignups   map[string]*signup.Signup
}

func (m *fakeSignupService) defaultMockGetSignup() func(userID, username string) (*signup.Signup, error) {
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

func (m *fakeSignupService) GetSignup(userID, username string) (*signup.Signup, error) {
	return m.mockGetSignup(userID, username)
}

func (m *fakeSignupService) Signup(_ *gin.Context) (*toolchainv1alpha1.UserSignup, error) {
	return nil, nil
}
func (m *fakeSignupService) GetUserSignup(_, _ string) (*toolchainv1alpha1.UserSignup, error) {
	return nil, nil
}
func (m *fakeSignupService) UpdateUserSignup(_ *toolchainv1alpha1.UserSignup) (*toolchainv1alpha1.UserSignup, error) {
	return nil, nil
}
func (m *fakeSignupService) PhoneNumberAlreadyInUse(_, _, _ string) error {
	return nil
}
