package service_test

import (
	"fmt"
	"testing"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	appservice "github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/informers"
	"github.com/codeready-toolchain/registration-service/pkg/informers/service"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
	"github.com/codeready-toolchain/registration-service/test"
	"github.com/codeready-toolchain/toolchain-common/pkg/hash"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

func TestRunInformerServiceSuite(t *testing.T) {
	suite.Run(t, &TestInformerServiceSuite{test.UnitTestSuite{}})
}

type TestInformerServiceSuite struct {
	test.UnitTestSuite
}

// Testing the Informer Service is mainly about ensuring the right types are being returned since informers use
// dynamic clients and if a Lister is setup incorrectly it can lead to bugs where we try to convert the unstructured object
// to the wrong type. This is possible and will not return any errors!
func (s *TestInformerServiceSuite) TestInformerService() {

	s.Run("masteruserrecords", func() {
		// given
		murLister := fakeLister{
			objs: map[string]*unstructured.Unstructured{
				"johnMur": {
					Object: map[string]interface{}{
						"spec": map[string]interface{}{
							"tierName": "deactivate30",
							"propagatedClaims": map[string]interface{}{
								"sub": "john-id",
							},
							"userAccounts": []map[string]interface{}{
								{
									"targetCluster": "member1",
								},
							},
						},
					},
				},
				"noise": {
					Object: map[string]interface{}{
						"spec": map[string]interface{}{
							"tierName": "deactivate30",
							"propagatedClaims": map[string]interface{}{
								"sub": "noise-id",
							},
							"userAccounts": []map[string]interface{}{
								{
									"targetCluster": "member2",
								},
							},
						},
					},
				},
			},
		}
		inf := informers.Informer{
			Masteruserrecord: murLister,
		}

		svc := service.NewInformerService(fakeInformerServiceContext{
			Svcs:     s.Application,
			informer: inf,
		})

		s.Run("not found", func() {
			// when
			val, err := svc.GetMasterUserRecord("unknown")

			//then
			assert.Nil(s.T(), val)
			assert.EqualError(s.T(), err, "not found")
		})

		s.Run("found", func() {
			// given
			expected := &toolchainv1alpha1.MasterUserRecord{
				Spec: toolchainv1alpha1.MasterUserRecordSpec{
					TierName: "deactivate30",
					UserAccounts: []toolchainv1alpha1.UserAccountEmbedded{
						{
							TargetCluster: "member1",
						},
					},
					PropagatedClaims: toolchainv1alpha1.PropagatedClaims{
						Sub: "john-id",
					},
				},
			}

			// when
			val, err := svc.GetMasterUserRecord("johnMur")

			// then
			require.NotNil(s.T(), val)
			require.NoError(s.T(), err)
			assert.Equal(s.T(), expected, val)
		})
	})

	s.Run("spaces", func() {
		// given
		spaceLister := fakeLister{
			objs: map[string]*unstructured.Unstructured{
				"johnSpace": {
					Object: map[string]interface{}{
						"spec": map[string]interface{}{
							"targetCluster": "member2",
							"tierName":      "base1ns",
						},
					},
				},
				"noise": {
					Object: map[string]interface{}{
						"spec": map[string]interface{}{
							"targetCluster": "member1",
							"tierName":      "base",
						},
					},
				},
			},
		}

		inf := informers.Informer{
			Space: spaceLister,
		}

		svc := service.NewInformerService(fakeInformerServiceContext{
			Svcs:     s.Application,
			informer: inf,
		})

		s.Run("not found", func() {
			// when
			val, err := svc.GetSpace("unknown")

			// then
			assert.Nil(s.T(), val)
			assert.EqualError(s.T(), err, "not found")
		})

		s.Run("found", func() {
			// given
			expected := &toolchainv1alpha1.Space{
				Spec: toolchainv1alpha1.SpaceSpec{
					TargetCluster: "member2",
					TierName:      "base1ns",
				},
			}

			// when
			val, err := svc.GetSpace("johnSpace")

			// then
			require.NotNil(s.T(), val)
			require.NoError(s.T(), err)
			assert.Equal(s.T(), expected, val)
		})
	})

	s.Run("proxy configs", func() {
		// given
		proxyConfigLister := fakeLister{
			objs: map[string]*unstructured.Unstructured{
				"tekton-results": {
					Object: map[string]interface{}{
						"spec": map[string]interface{}{
							"openShiftRouteTargetEndpoint": map[string]interface{}{
								"namespace": "tekton-results",
								"name":      "tekton-results",
							},
						},
					},
				},
				"noise": {
					Object: map[string]interface{}{
						"spec": map[string]interface{}{},
					},
				},
			},
		}

		inf := informers.Informer{
			ProxyPluginConfig: proxyConfigLister,
		}

		svc := service.NewInformerService(fakeInformerServiceContext{
			Svcs:     s.Application,
			informer: inf,
		})

		s.Run("not found", func() {
			// when
			val, err := svc.GetProxyPluginConfig("unknown")

			// then
			assert.Nil(s.T(), val)
			assert.EqualError(s.T(), err, "not found")
		})

		s.Run("found", func() {
			// given
			expected := &toolchainv1alpha1.ProxyPlugin{
				Spec: toolchainv1alpha1.ProxyPluginSpec{
					OpenShiftRouteTargetEndpoint: &toolchainv1alpha1.OpenShiftRouteTarget{
						Namespace: "tekton-results",
						Name:      "tekton-results",
					},
				},
			}

			// when
			val, err := svc.GetProxyPluginConfig("tekton-results")

			// then
			require.NotNil(s.T(), val)
			require.NoError(s.T(), err)
			assert.Equal(s.T(), expected, val)
		})
	})

	s.Run("toolchainstatuses", func() {
		// given
		emptyToolchainStatusLister := fakeLister{
			objs: map[string]*unstructured.Unstructured{},
		}

		toolchainStatusLister := fakeLister{
			objs: map[string]*unstructured.Unstructured{
				"toolchain-status": {
					Object: map[string]interface{}{
						"status": map[string]interface{}{
							"hostOperator": map[string]interface{}{
								"version": "v1alpha1",
							},
						},
					},
				},
			},
		}

		s.Run("not found", func() {
			// given
			inf := informers.Informer{
				ToolchainStatus: emptyToolchainStatusLister,
			}

			svc := service.NewInformerService(fakeInformerServiceContext{
				Svcs:     s.Application,
				informer: inf,
			})

			// when
			val, err := svc.GetToolchainStatus()

			// then
			assert.Nil(s.T(), val)
			assert.EqualError(s.T(), err, "not found")
		})

		s.Run("found", func() {
			// given
			inf := informers.Informer{
				ToolchainStatus: toolchainStatusLister,
			}

			svc := service.NewInformerService(fakeInformerServiceContext{
				Svcs:     s.Application,
				informer: inf,
			})

			expected := &toolchainv1alpha1.ToolchainStatus{
				Status: toolchainv1alpha1.ToolchainStatusStatus{
					HostOperator: &toolchainv1alpha1.HostOperatorStatus{
						Version: "v1alpha1",
					},
				},
			}

			// when
			val, err := svc.GetToolchainStatus()

			// then
			require.NotNil(s.T(), val)
			require.NoError(s.T(), err)
			assert.Equal(s.T(), expected, val)
		})
	})

	s.Run("usersignups", func() {
		// given
		userSignupLister := fakeLister{
			objs: map[string]*unstructured.Unstructured{
				"johnUserSignup": {
					Object: map[string]interface{}{
						"spec": map[string]interface{}{
							"targetCluster": "member2",
							"identityClaims": map[string]interface{}{
								"sub":               "foo",
								"originalSub":       "sub-key",
								"preferredUsername": "foo@redhat.com",
								"givenName":         "Foo",
								"familyName":        "Bar",
								"company":           "Red Hat",
							},
						},
					},
				},
				"noise": {
					Object: map[string]interface{}{
						"spec": map[string]interface{}{
							"targetCluster": "member1",
							"identityClaims": map[string]interface{}{
								"sub":               "noise",
								"originalSub":       "noise-key",
								"preferredUsername": "noise@redhat.com",
								"givenName":         "Noise",
								"familyName":        "Make",
								"company":           "Noisy",
							},
						},
					},
				},
			},
		}

		inf := informers.Informer{
			UserSignup: userSignupLister,
		}

		svc := service.NewInformerService(fakeInformerServiceContext{
			Svcs:     s.Application,
			informer: inf,
		})

		s.Run("not found", func() {
			// when
			val, err := svc.GetUserSignup("unknown")

			// then
			assert.Nil(s.T(), val)
			assert.EqualError(s.T(), err, "not found")
		})

		s.Run("found", func() {
			// given
			expected := &toolchainv1alpha1.UserSignup{
				Spec: toolchainv1alpha1.UserSignupSpec{
					TargetCluster: "member2",
					IdentityClaims: toolchainv1alpha1.IdentityClaimsEmbedded{
						PreferredUsername: "foo@redhat.com",
						GivenName:         "Foo",
						FamilyName:        "Bar",
						Company:           "Red Hat",
						PropagatedClaims: toolchainv1alpha1.PropagatedClaims{
							Sub:         "foo",
							OriginalSub: "sub-key",
						},
					},
				},
			}

			// when
			val, err := svc.GetUserSignup("johnUserSignup")

			// then
			require.NotNil(s.T(), val)
			require.NoError(s.T(), err)
			assert.Equal(s.T(), expected, val)
		})
	})

	s.Run("bannedusers", func() {
		// given
		bb := map[string]toolchainv1alpha1.BannedUser{
			"alice": {
				ObjectMeta: metav1.ObjectMeta{
					Name:      "alice",
					Namespace: configuration.Namespace(),
					Labels: map[string]string{
						toolchainv1alpha1.BannedUserEmailHashLabelKey: hash.EncodeString("alice@redhat.com"),
					},
				},
				Spec: toolchainv1alpha1.BannedUserSpec{
					Email: "alice@redhat.com",
				},
			},
			"bob": {
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bob",
					Namespace: configuration.Namespace(),
					Labels: map[string]string{
						toolchainv1alpha1.BannedUserEmailHashLabelKey: hash.EncodeString("bob@redhat.com"),
					},
				},
				Spec: toolchainv1alpha1.BannedUserSpec{
					Email: "bob@redhat.com",
				},
			},
			"bob-dup": {
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bob-dup",
					Namespace: configuration.Namespace(),
					Labels: map[string]string{
						toolchainv1alpha1.BannedUserEmailHashLabelKey: hash.EncodeString("bob@redhat.com"),
					},
				},
				Spec: toolchainv1alpha1.BannedUserSpec{
					Email: "bob@redhat.com",
				},
			},
		}

		// convert to unstructured.Unstructured for fakeLister
		bbu := make(map[string]*unstructured.Unstructured, len(bb))
		for k, b := range bb {
			bu, err := runtime.DefaultUnstructuredConverter.ToUnstructured(b.DeepCopy())
			require.NoError(s.T(), err)
			bbu[k] = &unstructured.Unstructured{Object: bu}
		}

		// setup InformerService with fakeLister
		inf := informers.Informer{BannedUsers: fakeLister{objs: bbu}}
		svc := service.NewInformerService(fakeInformerServiceContext{
			Svcs:     s.Application,
			informer: inf,
		})

		s.Run("not found", func() {
			// when
			rbb, err := svc.ListBannedUsersByEmail("unknown@unknown.com")

			// then
			require.NoError(s.T(), err)
			require.Empty(s.T(), rbb)
		})

		s.Run("invalid email", func() {
			// when
			rbb, err := svc.ListBannedUsersByEmail("not-an-email")

			// then
			require.NoError(s.T(), err)
			require.Empty(s.T(), rbb)
		})

		s.Run("found one", func() {
			// given
			expected := bb["alice"]

			// when
			rbb, err := svc.ListBannedUsersByEmail(expected.Spec.Email)

			// then
			require.NotNil(s.T(), rbb)
			require.NoError(s.T(), err)
			require.Len(s.T(), rbb, 1, "expected 1 result for email %s", expected.Spec.Email)
			require.Equal(s.T(), expected, rbb[0])
		})

		s.Run("found multiple", func() {
			// given
			expected := []toolchainv1alpha1.BannedUser{bb["bob"], bb["bob-dup"]}

			// when
			rbb, err := svc.ListBannedUsersByEmail(expected[0].Spec.Email)

			// then
			require.NotNil(s.T(), rbb)
			require.NoError(s.T(), err)
			require.Len(s.T(), rbb, 2, "expected 2 results for email %s", expected[0].Spec.Email)
			require.Equal(s.T(), expected, rbb)
		})
	})
}

type fakeInformerServiceContext struct {
	Svcs     appservice.Services
	informer informers.Informer
}

func (sc fakeInformerServiceContext) CRTClient() kubeclient.CRTClient {
	panic("shouldn't need CRTClient")
}

func (sc fakeInformerServiceContext) Informer() informers.Informer {
	return sc.informer
}

func (sc fakeInformerServiceContext) Services() appservice.Services {
	return sc.Svcs
}

type fakeLister struct {
	objs map[string]*unstructured.Unstructured
}

// List will return all objects across namespaces
func (l fakeLister) List(ls labels.Selector) (ret []runtime.Object, err error) {
	objs := []runtime.Object{}
	for _, o := range l.objs {
		ll := labels.Set(o.GetLabels())
		if ls.Matches(ll) {
			objs = append(objs, o)
		}
	}
	return objs, nil
}

// Get will attempt to retrieve assuming that name==key
func (l fakeLister) Get(name string) (runtime.Object, error) {
	obj := l.objs[name]
	if obj != nil {
		return obj, nil
	}
	return nil, fmt.Errorf("not found")
}

// ByNamespace will give you a GenericNamespaceLister for one namespace
func (l fakeLister) ByNamespace(_ string) cache.GenericNamespaceLister {
	return l
}
