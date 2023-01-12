package service_test

import (
	"fmt"
	"testing"

	appservice "github.com/codeready-toolchain/registration-service/pkg/application/service"
	"github.com/codeready-toolchain/registration-service/pkg/informers"
	"github.com/codeready-toolchain/registration-service/pkg/informers/service"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
	"github.com/codeready-toolchain/registration-service/test"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

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
		murLister := fakeLister{
			objs: map[string]*unstructured.Unstructured{
				"johnMur": {
					Object: map[string]interface{}{
						"spec": map[string]interface{}{
							"tierName": "deactivate30",
							"userID":   "john-id",
							"userAccounts": []map[string]interface{}{
								{
									"targetCluster": "member1",
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
			val, err := svc.GetMasterUserRecord("unknown")
			assert.Nil(s.T(), val)
			assert.EqualError(s.T(), err, "not found")
		})

		s.Run("found", func() {
			expected := &toolchainv1alpha1.MasterUserRecord{
				Spec: toolchainv1alpha1.MasterUserRecordSpec{
					TierName: "deactivate30",
					UserID:   "john-id",
					UserAccounts: []toolchainv1alpha1.UserAccountEmbedded{
						{
							TargetCluster: "member1",
						},
					},
				},
			}

			val, err := svc.GetMasterUserRecord("johnMur")
			require.NotNil(s.T(), val)
			require.NoError(s.T(), err)
			assert.Equal(s.T(), val, expected)
		})
	})

	s.Run("toolchainstatuses", func() {
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
			inf := informers.Informer{
				ToolchainStatus: emptyToolchainStatusLister,
			}

			svc := service.NewInformerService(fakeInformerServiceContext{
				Svcs:     s.Application,
				informer: inf,
			})
			val, err := svc.GetToolchainStatus()
			assert.Nil(s.T(), val)
			assert.EqualError(s.T(), err, "not found")
		})

		s.Run("found", func() {
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

			val, err := svc.GetToolchainStatus()
			require.NotNil(s.T(), val)
			require.NoError(s.T(), err)
			assert.Equal(s.T(), val, expected)
		})
	})

	s.Run("usersignups", func() {

		userSignupLister := fakeLister{
			objs: map[string]*unstructured.Unstructured{
				"johnUserSignup": {
					Object: map[string]interface{}{
						"spec": map[string]interface{}{
							"targetCluster": "member2",
							"username":      "foo@redhat.com",
							"userid":        "foo",
							"givenName":     "Foo",
							"familyName":    "Bar",
							"company":       "Red Hat",
							"originalSub":   "sub-key",
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
			val, err := svc.GetUserSignup("unknown")
			assert.Nil(s.T(), val)
			assert.EqualError(s.T(), err, "not found")
		})

		s.Run("found", func() {
			expected := &toolchainv1alpha1.UserSignup{
				Spec: toolchainv1alpha1.UserSignupSpec{
					TargetCluster: "member2",
					Username:      "foo@redhat.com",
					Userid:        "foo",
					GivenName:     "Foo",
					FamilyName:    "Bar",
					Company:       "Red Hat",
					OriginalSub:   "sub-key",
				},
			}

			val, err := svc.GetUserSignup("johnUserSignup")
			require.NotNil(s.T(), val)
			require.NoError(s.T(), err)
			assert.Equal(s.T(), val, expected)
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
func (l fakeLister) List(selector labels.Selector) (ret []runtime.Object, err error) {
	return nil, nil
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
func (l fakeLister) ByNamespace(namespace string) cache.GenericNamespaceLister {
	return l
}
