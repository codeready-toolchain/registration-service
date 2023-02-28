package service_test

import (
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/proxy/access"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/service"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/registration-service/test"
	"github.com/codeready-toolchain/registration-service/test/fake"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	commoncluster "github.com/codeready-toolchain/toolchain-common/pkg/cluster"
	commontest "github.com/codeready-toolchain/toolchain-common/pkg/test"

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
	sc := fake.NewSignupService(fake.Signup("123-noise", &signup.Signup{
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
	s.Application.MockSignupService(sc)

	inf := fake.NewFakeInformer()
	inf.GetSpaceFunc = func(name string) (*toolchainv1alpha1.Space, error) {
		switch name {
		case "noise1", "teamspace":
			return fake.NewSpace(name, "member-1", name), nil
		case "smith2":
			return fake.NewSpace(name, "member-2", name), nil
		case "unknown-cluster":
			return fake.NewSpace(name, "unknown-cluster", name), nil
		}
		return nil, fmt.Errorf("space not found error")
	}
	s.Application.MockInformerService(inf)

	svc := service.NewMemberClusterService(
		fake.MemberClusterServiceContext{
			Client: s,
			Svcs:   s.Application,
		},
	)

	s.Run("unable to get signup", func() {
		s.Run("signup service returns error", func() {
			sc.MockGetSignup = func(userID, username string) (*signup.Signup, error) {
				return nil, errors.New("oopsi woopsi")
			}

			// when
			_, err := svc.GetClusterAccess("789-ready", "", "")

			// then
			require.EqualError(s.T(), err, "oopsi woopsi")
		})

		sc.MockGetSignup = sc.DefaultMockGetSignup() // restore the default signup service, so it doesn't return an error anymore

		s.Run("userid is not found", func() {
			// when
			_, err := svc.GetClusterAccess("unknown_id", "", "")

			// then
			require.EqualError(s.T(), err, "user is not provisioned (yet)")
		})

		s.Run("username is not found", func() {
			// when
			_, err := svc.GetClusterAccess("", "unknown_username", "")

			// then
			require.EqualError(s.T(), err, "user is not provisioned (yet)")
		})

		s.Run("user is not provisioned yet", func() {
			// when
			_, err := svc.GetClusterAccess("456-not-ready", "", "")

			// then
			require.EqualError(s.T(), err, "user is not provisioned (yet)")
		})
	})

	s.Run("unable to get space", func() {
		s.Run("informer service returns error", func() {
			original := inf.GetSpaceFunc
			defer func() { // restore original GetSpaceFunc after test
				inf.GetSpaceFunc = original
				s.Application.MockInformerService(inf)
			}()
			inf.GetSpaceFunc = func(name string) (*toolchainv1alpha1.Space, error) { // informer error
				return nil, fmt.Errorf("oopsi woopsi")
			}
			s.Application.MockInformerService(inf)

			// when
			_, err := svc.GetClusterAccess("789-ready", "", "smith2")

			// then
			// original error is only logged so that it doesn't reveal information about a space that may not belong to the requestor
			require.EqualError(s.T(), err, "the requested space is not available")
		})

		s.Run("space not found", func() {
			// when
			_, err := svc.GetClusterAccess("789-ready", "", "unknown") // unknown workspace requested

			// then
			require.EqualError(s.T(), err, "the requested space is not available")
		})
	})

	s.Run("no member cluster found", func() {
		s.Run("no member clusters", func() {
			svc := service.NewMemberClusterService(
				fake.MemberClusterServiceContext{
					Client: s,
					Svcs:   s.Application,
				},
				func(si *service.ServiceImpl) {
					si.GetMembersFunc = func(conditions ...commoncluster.Condition) []*commoncluster.CachedToolchainCluster {
						return []*commoncluster.CachedToolchainCluster{}
					}
				},
			)
			s.Run("default workspace case", func() {
				// when
				_, err := svc.GetClusterAccess("789-ready", "", "")

				// then
				require.EqualError(s.T(), err, "no member clusters found")
			})

			s.Run("workspace context case", func() {
				// when
				_, err := svc.GetClusterAccess("789-ready", "", "smith2")

				// then
				require.EqualError(s.T(), err, "no member clusters found")
			})
		})

		s.Run("no member cluster with the given URL", func() {
			svc := service.NewMemberClusterService(
				fake.MemberClusterServiceContext{
					Client: s,
					Svcs:   s.Application,
				},
				func(si *service.ServiceImpl) {
					si.GetMembersFunc = func(conditions ...commoncluster.Condition) []*commoncluster.CachedToolchainCluster {
						return s.memberClusters()
					}
				},
			)

			s.Run("default workspace case", func() {
				// when
				_, err := svc.GetClusterAccess("012-ready-unknown-cluster", "", "")

				// then
				require.EqualError(s.T(), err, "no member cluster found for the user")
			})

			s.Run("workspace context case", func() {
				// when
				_, err := svc.GetClusterAccess("012-ready-unknown-cluster", "", "unknown-cluster")

				// then
				require.EqualError(s.T(), err, "no member cluster found for space 'unknown-cluster'")
			})
		})
	})

	s.Run("member found", func() {
		memberClient := commontest.NewFakeClient(s.T())

		svc := service.NewMemberClusterService(
			fake.MemberClusterServiceContext{
				Client: s,
				Svcs:   s.Application,
			},
			func(si *service.ServiceImpl) {
				si.GetMembersFunc = func(conditions ...commoncluster.Condition) []*commoncluster.CachedToolchainCluster {
					return []*commoncluster.CachedToolchainCluster{
						{
							Config: &commoncluster.Config{
								Name:        "member-1",
								Type:        commoncluster.Member,
								APIEndpoint: "https://api.endpoint.member-1.com:6443",
								RestConfig: &rest.Config{
									BearerToken: "def456",
								},
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
			ca, err := svc.GetClusterAccess("789-ready", "", "")

			// then
			require.NoError(s.T(), err)
			require.NotNil(s.T(), ca)
			expectedURL, err := url.Parse("https://api.endpoint.member-2.com:6443")
			require.NoError(s.T(), err)
			assert.Equal(s.T(), "smith2", ca.Username())

			s.assertClusterAccess(access.NewClusterAccess(*expectedURL, expectedToken, ""), ca)

			s.Run("cluster access correct when username provided", func() {
				// when
				ca, err := svc.GetClusterAccess("", "smith@", "")

				// then
				require.NoError(s.T(), err)
				require.NotNil(s.T(), ca)
				expectedURL, err := url.Parse("https://api.endpoint.member-2.com:6443")
				require.NoError(s.T(), err)
				s.assertClusterAccess(access.NewClusterAccess(*expectedURL, expectedToken, "smith"), ca)
				assert.Equal(s.T(), "smith2", ca.Username())
			})

			s.Run("cluster access correct when using workspace context", func() {
				// when
				ca, err := svc.GetClusterAccess("789-ready", "", "smith2") // workspace-context specified

				// then
				require.NoError(s.T(), err)
				require.NotNil(s.T(), ca)
				expectedURL, err := url.Parse("https://api.endpoint.member-2.com:6443")
				require.NoError(s.T(), err)
				s.assertClusterAccess(access.NewClusterAccess(*expectedURL, expectedToken, "smith"), ca)
				assert.Equal(s.T(), "smith2", ca.Username())

				s.Run("another workspace on another cluster", func() {
					// when
					ca, err := svc.GetClusterAccess("789-ready", "", "teamspace") // workspace-context specified

					// then
					require.NoError(s.T(), err)
					require.NotNil(s.T(), ca)
					expectedURL, err := url.Parse("https://api.endpoint.member-1.com:6443")
					require.NoError(s.T(), err)
					s.assertClusterAccess(access.NewClusterAccess(*expectedURL, "def456", "smith"), ca)
					assert.Equal(s.T(), "smith2", ca.Username())
				})
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
