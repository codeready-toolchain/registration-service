package service_test

import (
	"context"
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

	routev1 "github.com/openshift/api/route/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		CompliantUsername: "",
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
	inf.GetProxyPluginConfigFunc = func(name string) (*toolchainv1alpha1.ProxyPlugin, error) {
		if name != "tekton-results" {
			return nil, errors.New("not found")
		}
		pp := &toolchainv1alpha1.ProxyPlugin{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: toolchainv1alpha1.ProxyPluginSpec{
				OpenShiftRouteTargetEndpoint: &toolchainv1alpha1.OpenShiftRouteTarget{
					Namespace: "tekton-results",
					Name:      "tekton-results",
				},
			},
		}
		return pp, nil
	}
	s.Application.MockInformerService(inf)

	svc := service.NewMemberClusterService(
		fake.MemberClusterServiceContext{
			Client: s,
			Svcs:   s.Application,
		},
	)

	tt := map[string]struct {
		publicViewerEnabled bool
	}{
		"public-viewer enabled":  {publicViewerEnabled: true},
		"public-viewer disabled": {publicViewerEnabled: false},
	}

	for k, tc := range tt {
		publicViewerEnabled := tc.publicViewerEnabled

		s.Run(k, func() {

			s.Run("unable to get signup", func() {
				tt := map[string]struct {
					workspace string
				}{
					"default workspace":      {workspace: ""},
					"not-existing workspace": {workspace: "not-existing"},
				}
				for k, tc := range tt {
					s.Run(k, func() {
						s.Run("signup service returns error", func() {
							sc.MockGetSignup = func(_, _ string) (*signup.Signup, error) {
								return nil, errors.New("oopsi woopsi")
							}

							// when
							_, err := svc.GetClusterAccess("789-ready", "", tc.workspace, "", publicViewerEnabled)

							// then
							require.EqualError(s.T(), err, "oopsi woopsi")
						})

						sc.MockGetSignup = sc.DefaultMockGetSignup() // restore the default signup service, so it doesn't return an error anymore

						s.Run("userid is not found", func() {
							// when
							_, err := svc.GetClusterAccess("unknown_id", "", tc.workspace, "", publicViewerEnabled)

							// then
							require.EqualError(s.T(), err, "user is not provisioned (yet)")
						})

						s.Run("username is not found", func() {
							// when
							_, err := svc.GetClusterAccess("", "unknown_username", tc.workspace, "", publicViewerEnabled)

							// then
							require.EqualError(s.T(), err, "user is not provisioned (yet)")
						})

						s.Run("user is not provisioned yet", func() {
							// when
							_, err := svc.GetClusterAccess("456-not-ready", "", tc.workspace, "", publicViewerEnabled)

							// then
							require.EqualError(s.T(), err, "user is not provisioned (yet)")
						})

					})
				}
			})

			s.Run("unable to get space", func() {
				s.Run("informer service returns error", func() {
					original := inf.GetSpaceFunc
					defer func() { // restore original GetSpaceFunc after test
						inf.GetSpaceFunc = original
						s.Application.MockInformerService(inf)
					}()
					inf.GetSpaceFunc = func(_ string) (*toolchainv1alpha1.Space, error) { // informer error
						return nil, fmt.Errorf("oopsi woopsi")
					}
					s.Application.MockInformerService(inf)

					// when
					_, err := svc.GetClusterAccess("789-ready", "", "smith2", "", publicViewerEnabled)

					// then
					// original error is only logged so that it doesn't reveal information about a space that may not belong to the requestor
					require.EqualError(s.T(), err, "the requested space is not available")
				})

				s.Run("space not found", func() {
					// when
					_, err := svc.GetClusterAccess("789-ready", "", "unknown", "", publicViewerEnabled) // unknown workspace requested

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
							si.GetMembersFunc = func(_ ...commoncluster.Condition) []*commoncluster.CachedToolchainCluster {
								return []*commoncluster.CachedToolchainCluster{}
							}
						},
					)
					s.Run("default workspace case", func() {
						// when
						_, err := svc.GetClusterAccess("789-ready", "", "", "", publicViewerEnabled)

						// then
						require.EqualError(s.T(), err, "no member clusters found")
					})

					s.Run("workspace context case", func() {
						// when
						_, err := svc.GetClusterAccess("789-ready", "", "smith2", "", publicViewerEnabled)

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
							si.GetMembersFunc = func(_ ...commoncluster.Condition) []*commoncluster.CachedToolchainCluster {
								return s.memberClusters()
							}
						},
					)

					s.Run("default workspace case", func() {
						// when
						_, err := svc.GetClusterAccess("012-ready-unknown-cluster", "", "", "", publicViewerEnabled)

						// then
						require.EqualError(s.T(), err, "no member cluster found for the user")
					})

					s.Run("workspace context case", func() {
						// when
						_, err := svc.GetClusterAccess("012-ready-unknown-cluster", "", "unknown-cluster", "", publicViewerEnabled)

						// then
						require.EqualError(s.T(), err, "no member cluster found for space 'unknown-cluster'")
					})
				})
			})

			s.Run("member found", func() {
				memberClient := commontest.NewFakeClient(s.T())
				memberArray := []*commoncluster.CachedToolchainCluster{
					{
						Config: &commoncluster.Config{
							Name:        "member-1",
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
							RestConfig:  &rest.Config{},
						},
					},
				}

				svc := service.NewMemberClusterService(
					fake.MemberClusterServiceContext{
						Client: s,
						Svcs:   s.Application,
					},
					func(si *service.ServiceImpl) {
						si.GetMembersFunc = func(_ ...commoncluster.Condition) []*commoncluster.CachedToolchainCluster {
							return memberArray
						}
					},
				)

				s.Run("verify cluster access with route", func() {
					memberClient.MockGet = func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						route, ok := obj.(*routev1.Route)
						if ok && key.Namespace == "tekton-results" && key.Name == "tekton-results" {
							route.Namespace = key.Namespace
							route.Name = key.Name
							route.Status.Ingress = []routev1.RouteIngress{
								{
									Host: "myservice.endpoint.member-2.com",
								},
							}
							return nil
						}
						return memberClient.Client.Get(ctx, key, obj, opts...)
					}
					expectedToken := "abc123" // should match member 2 bearer token

					// when
					ca, err := svc.GetClusterAccess("789-ready", "", "", "tekton-results", publicViewerEnabled)

					// then
					require.NoError(s.T(), err)
					require.NotNil(s.T(), ca)
					expectedURL, err := url.Parse("https://myservice.endpoint.member-2.com")
					require.NoError(s.T(), err)
					assert.Equal(s.T(), "smith2", ca.Username())

					s.assertClusterAccess(access.NewClusterAccess(*expectedURL, expectedToken, ""), ca)

					s.Run("cluster access correct when username provided", func() {
						// when
						ca, err := svc.GetClusterAccess("", "smith@", "", "tekton-results", publicViewerEnabled)

						// then
						require.NoError(s.T(), err)
						require.NotNil(s.T(), ca)
						expectedURL, err := url.Parse("https://myservice.endpoint.member-2.com")
						require.NoError(s.T(), err)
						s.assertClusterAccess(access.NewClusterAccess(*expectedURL, expectedToken, "smith"), ca)
						assert.Equal(s.T(), "smith2", ca.Username())
					})

					s.Run("cluster access correct when using workspace context", func() {
						// when
						ca, err := svc.GetClusterAccess("789-ready", "", "smith2", "tekton-results", publicViewerEnabled) // workspace-context specified

						// then
						require.NoError(s.T(), err)
						require.NotNil(s.T(), ca)
						expectedURL, err := url.Parse("https://myservice.endpoint.member-2.com")
						require.NoError(s.T(), err)
						s.assertClusterAccess(access.NewClusterAccess(*expectedURL, expectedToken, "smith"), ca)
						assert.Equal(s.T(), "smith2", ca.Username())

						s.Run("another workspace on another cluster", func() {
							// when
							mC := commontest.NewFakeClient(s.T())
							mC.MockGet = func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
								route, ok := obj.(*routev1.Route)
								if ok && key.Namespace == "tekton-results" && key.Name == "tekton-results" {
									route.Namespace = key.Namespace
									route.Name = key.Name
									route.Status.Ingress = []routev1.RouteIngress{
										{
											Host: "api.endpoint.member-1.com:6443",
										},
									}
									return nil
								}
								return memberClient.Client.Get(ctx, key, obj, opts...)
							}
							memberArray[0].Client = mC
							ca, err := svc.GetClusterAccess("789-ready", "", "teamspace", "tekton-results", publicViewerEnabled) // workspace-context specified

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

				s.Run("verify cluster access no route", func() {
					memberClient.MockGet = nil
					expectedToken := "abc123" // should match member 2 bearer token

					// when
					ca, err := svc.GetClusterAccess("789-ready", "", "", "", publicViewerEnabled)

					// then
					require.NoError(s.T(), err)
					require.NotNil(s.T(), ca)
					expectedURL, err := url.Parse("https://api.endpoint.member-2.com:6443")
					require.NoError(s.T(), err)
					assert.Equal(s.T(), "smith2", ca.Username())

					s.assertClusterAccess(access.NewClusterAccess(*expectedURL, expectedToken, ""), ca)

					s.Run("cluster access correct when username provided", func() {
						// when
						ca, err := svc.GetClusterAccess("", "smith@", "", "", publicViewerEnabled)

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
						ca, err := svc.GetClusterAccess("789-ready", "", "smith2", "", publicViewerEnabled) // workspace-context specified

						// then
						require.NoError(s.T(), err)
						require.NotNil(s.T(), ca)
						expectedURL, err := url.Parse("https://api.endpoint.member-2.com:6443")
						require.NoError(s.T(), err)
						s.assertClusterAccess(access.NewClusterAccess(*expectedURL, expectedToken, "smith"), ca)
						assert.Equal(s.T(), "smith2", ca.Username())

						s.Run("another workspace on another cluster", func() {
							// when
							ca, err := svc.GetClusterAccess("789-ready", "", "teamspace", "", publicViewerEnabled) // workspace-context specified

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
		})
	}

	// public-viewer enabled
	s.Run("user is public-viewer", func() {
		s.Run("always get space", func() {
			//given
			expectedURL, err := url.Parse("https://api.endpoint.member-2.com:6443")
			expectedClusterAccess := access.NewClusterAccess(*expectedURL, "token", toolchainv1alpha1.KubesawAuthenticatedUsername)
			svc := service.NewMemberClusterService(
				fake.MemberClusterServiceContext{
					Client: s,
					Svcs:   s.Application,
				},
				func(si *service.ServiceImpl) {
					si.GetMembersFunc = func(_ ...commoncluster.Condition) []*commoncluster.CachedToolchainCluster {
						return s.memberClusters()
					}
				},
			)

			// when
			clusterAccess, err := svc.GetClusterAccess("", toolchainv1alpha1.KubesawAuthenticatedUsername, "smith2", "", true)

			// then
			require.NoError(s.T(), err)
			require.Equal(s.T(), expectedClusterAccess, clusterAccess)
		})

		s.Run("has no default workspace", func() {
			// when
			_, err := svc.GetClusterAccess("", toolchainv1alpha1.KubesawAuthenticatedUsername, "", "", true)

			// then
			require.EqualError(s.T(), err, "user is not provisioned (yet)")
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
				OperatorNamespace: "member-operator",
				RestConfig: &rest.Config{
					BearerToken: "token",
				},
			},
			Client: nil,
		})
	}
	return cls
}
