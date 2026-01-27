package namespaces

import (
	gocontext "context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/namespaced"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/registration-service/test"
	"github.com/codeready-toolchain/registration-service/test/fake"
	"github.com/codeready-toolchain/toolchain-common/pkg/cluster"
	commontest "github.com/codeready-toolchain/toolchain-common/pkg/test"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// ErrKubernetes is a generic error used for the fake client for when we
	// need to simulate error responses from Kubernetes.
	ErrKubernetes = errors.New("some generic Kubernetes error")
	// ErrUnableFetchUserSignup is an error that the fake client can return
	// when simulating that the user signup cannot be fetched.
	ErrUnableFetchUserSignup = errors.New("unable to fetch the user signup")
	// ErrUnableFetchSpace is an error that the fake client can return when
	// simulating that the user's space cannot be fetched.
	ErrUnableFetchSpace = errors.New("unable to fetch the space")
	// ErrUnableFetchNSTemplateSet is an error that the fake client can return
	// when simulating that the NSTemplateSet cannot be fetched
	ErrUnableFetchNSTemplateSet = errors.New("unable to fetch the NSTemplateSet")
)

// TestUsername represents the "user" the resources are bound to.
const TestUsername string = "DeveloperSandboxUser"

// TestNamespacesManagerSuite holds the unit test suite to be able to run the
// tests and the test fixtures that will be used throughout the tests.
type TestNamespacesManagerSuite struct {
	test.UnitTestSuite
}

func TestRunNamespacesManagerSuite(t *testing.T) {
	suite.Run(t, &TestNamespacesManagerSuite{test.UnitTestSuite{}})
}

// getMemberClusters is a mocked function that applies the given conditions to
// the member cluster fixtures that are generated in the tests, which avoids
// having to make real calls to Kubernetes.
func getMemberClusters(memberClusters []*cluster.CachedToolchainCluster) cluster.GetMemberClustersFunc {
	return func(conditions ...cluster.Condition) []*cluster.CachedToolchainCluster {
		result := make([]*cluster.CachedToolchainCluster, 0)

		for _, memberCluster := range memberClusters {
			for _, condition := range conditions {
				if condition(memberCluster) {
					result = append(result, memberCluster)
				}
			}
		}

		return result
	}
}

// createNewMemberCluster is a helper function to more easily create member
// cluster fixtures for our tests.
func createNewMemberCluster(client namespaced.Client, clusterName string) *cluster.CachedToolchainCluster {
	return &cluster.CachedToolchainCluster{
		Client: client,
		Config: &cluster.Config{
			Name: clusterName,
		},
	}
}

// mockClientGet is a helper function for the fake clients, which allows
// returning either a "proper expected response" from a hypothetical "GET"
// request, or a corresponding error depending on the object that we were
// supposed to be requesting.
//
// There is no argument for "user signup error" because we don't need it in
// the tests.
func mockClientGet(mockUserSignup *toolchainv1alpha1.UserSignup, mockSpace *toolchainv1alpha1.Space, spaceErr error, mockNSTemplateSet *toolchainv1alpha1.NSTemplateSet, nsTemplateSetErr error) func(gocontext.Context, client.ObjectKey, client.Object, ...client.GetOption) error {
	return func(_ gocontext.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
		if userSignup, ok := obj.(*toolchainv1alpha1.UserSignup); ok {
			*userSignup = *mockUserSignup
			return nil
		}

		if spaceErr != nil {
			return spaceErr
		} else if space, ok := obj.(*toolchainv1alpha1.Space); ok {
			*space = *mockSpace
			return nil
		}

		if nsTemplateSetErr != nil {
			return nsTemplateSetErr
		} else if nsTemplateSet, ok := obj.(*toolchainv1alpha1.NSTemplateSet); ok {
			*nsTemplateSet = *mockNSTemplateSet
			return nil
		}

		return ErrKubernetes
	}
}

// TestResetNamespaces provides multiple unit tests which test the different
// paths of the "ResetNamespaces" feature of the "NamespacesManager" type.
func (nms *TestNamespacesManagerSuite) TestResetNamespaces() {
	// Prepare a request for Gin. Even though we are testing the underlying
	// service, we do pull the original context from the request to apply
	// timeouts in the service, so it needs to be there.
	req, err := http.NewRequest(http.MethodPost, "/api/v1/reset-namespaces", nil)
	if err != nil {
		nms.Fail("unable to create test request", err.Error())
		return
	}

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Request = req
	ctx.Set(context.UsernameKey, TestUsername)

	// Create the fake client that will simulate the calls to Kubernetes, a
	// fake signup service and the service under test.
	fakeClient := commontest.NewFakeClient(nms.T())
	nsClient := namespaced.NewClient(fakeClient, "hehe")
	fakeSignupService := fake.NewSignupService([]*signup.Signup{{
		Name:              TestUsername,
		Username:          TestUsername,
		CompliantUsername: TestUsername,
		Status: signup.Status{
			Ready: true,
		},
	}}...)
	fakeSignupService.MockGetSignup = fakeSignupService.DefaultMockGetSignup()

	memberClusters := make([]*cluster.CachedToolchainCluster, 0)
	memberClusters = append(memberClusters, createNewMemberCluster(nsClient, "member-cluster-1"))
	memberClusters = append(memberClusters, createNewMemberCluster(nsClient, "member-cluster-2"))
	memberClusters = append(memberClusters, createNewMemberCluster(nsClient, "member-cluster-3"))

	namespacesManager := NewNamespacesManager(getMemberClusters(memberClusters), nsClient, fakeSignupService)

	// Create all the standard fixtures for the tests.
	userSignup := &toolchainv1alpha1.UserSignup{Status: toolchainv1alpha1.UserSignupStatus{CompliantUsername: TestUsername}}
	userSpace := &toolchainv1alpha1.Space{Spec: toolchainv1alpha1.SpaceSpec{TargetCluster: "member-cluster-2"}}

	namespace := "namespace-1"
	namespace2 := "namespace-2"
	namespace3 := "namespace-3"

	userNSTemplateSet := &toolchainv1alpha1.NSTemplateSet{Status: toolchainv1alpha1.NSTemplateSetStatus{ProvisionedNamespaces: []toolchainv1alpha1.SpaceNamespace{}}}
	userNSTemplateSet.Status.ProvisionedNamespaces = append(userNSTemplateSet.Status.ProvisionedNamespaces, toolchainv1alpha1.SpaceNamespace{Name: namespace})
	userNSTemplateSet.Status.ProvisionedNamespaces = append(userNSTemplateSet.Status.ProvisionedNamespaces, toolchainv1alpha1.SpaceNamespace{Name: namespace2})
	userNSTemplateSet.Status.ProvisionedNamespaces = append(userNSTemplateSet.Status.ProvisionedNamespaces, toolchainv1alpha1.SpaceNamespace{Name: namespace3})

	nms.Run("the user signup service returns an error", func() {
		// given
		// Simulate an error when attempting to fetch the user's signup
		// resource.
		fakeSignupService.MockGetSignup = func(_ string) (*signup.Signup, error) {
			return nil, ErrUnableFetchUserSignup
		}
		nms.T().Cleanup(func() { fakeSignupService.MockGetSignup = fakeSignupService.DefaultMockGetSignup() })

		// when
		// Call the function under test.
		err := namespacesManager.ResetNamespaces(ctx)

		// then
		// Assert that the returned error is the expected one.
		assert.Equal(nms.T(), fmt.Sprintf("unable to obtain the user signup: %s", ErrUnableFetchUserSignup.Error()), err.Error())
	})

	nms.Run(`the signup service returns a "not found or deactivated" response`, func() {
		// given
		// Simulate that the user service returns a "not found" or "inactive"
		// response by first returning a "nil" user signup, and then some user
		// signups with blank compliant usernames.
		testCases := []struct {
			signup *signup.Signup
		}{
			{signup: nil},
			{signup: &signup.Signup{CompliantUsername: ""}},
			{signup: &signup.Signup{CompliantUsername: "     "}},
		}

		nms.T().Cleanup(func() { fakeSignupService.MockGetSignup = fakeSignupService.DefaultMockGetSignup() })
		for _, testCase := range testCases {
			fakeSignupService.MockGetSignup = func(_ string) (*signup.Signup, error) {
				return testCase.signup, nil
			}

			// when
			// Call the function under test.
			err := namespacesManager.ResetNamespaces(ctx)

			// then
			// Assert that the returned error is the expected one.
			var targetErr ErrUserSignUpNotFoundDeactivated
			assert.ErrorAs(nms.T(), err, &targetErr)
		}
	})

	nms.Run("fetching user space returns error", func() {
		// given
		// Simulate that the call to fetch the user signup resource is
		// successful, but that there is an error when attempting to fetch
		// the user's space.
		fakeClient.MockGet = mockClientGet(userSignup, nil, ErrUnableFetchSpace, nil, nil)
		nms.T().Cleanup(func() { fakeClient.MockGet = nil })

		// when
		// Call the function under test.
		err := namespacesManager.ResetNamespaces(ctx)

		// then
		// Assert that the returned error is the expected one.
		assert.Equal(nms.T(), fmt.Sprintf("unable to get user's space resource: %s", ErrUnableFetchSpace.Error()), err.Error())
	})

	nms.Run("not being able to locate the cluster the user has been provisioned in returns error", func() {
		// given
		// Simulate that the calls to fetch the user signup and the user space
		// resources are successful, but that the latter returns a cluster
		// that will not generate a match.
		inexistentClusterNameSpace := &toolchainv1alpha1.Space{
			Spec: toolchainv1alpha1.SpaceSpec{
				TargetCluster: "inexistent-cluster",
			},
		}
		fakeClient.MockGet = mockClientGet(userSignup, inexistentClusterNameSpace, nil, nil, nil)
		nms.T().Cleanup(func() { fakeClient.MockGet = nil })

		// when
		// Call the function under test.
		err := namespacesManager.ResetNamespaces(ctx)

		// then
		// Assert that the returned error is the expected one.
		assert.Equal(nms.T(), fmt.Sprintf(`unable to locate the target cluster "%s" for the user`, inexistentClusterNameSpace.Spec.TargetCluster), err.Error())
	})

	nms.Run("a member cluster without a client will return an error", func() {
		// given
		// Return the user's signup and space resources when the manager is
		// requesting them.
		fakeClient.MockGet = mockClientGet(userSignup, userSpace, nil, nil, nil)
		nms.T().Cleanup(func() { fakeClient.MockGet = nil })

		// Simulate that the user's target cluster does not have a client.
		memberClusters[1].Client = nil
		nms.T().Cleanup(func() { memberClusters[1].Client = nsClient })

		// when
		// Call the function under test.
		err := namespacesManager.ResetNamespaces(ctx)

		// then
		// Assert that the returned error is the expected one.
		assert.Equal(nms.T(), fmt.Sprintf(`unable to obtain the client for cluster "%s"`, memberClusters[1].Name), err.Error())
	})

	nms.Run("not being able to fetch the user's NSTemplateSet will return an error", func() {
		// given
		// Simulate that fetching both the user signup and user space works,
		// but that fetching the "NSTemplateSet" gives an error.
		fakeClient.MockGet = mockClientGet(userSignup, userSpace, nil, nil, ErrUnableFetchNSTemplateSet)
		nms.T().Cleanup(func() { fakeClient.MockGet = nil })

		// when
		// Call the function under test.
		err := namespacesManager.ResetNamespaces(ctx)

		// then
		// Assert that the returned error is the expected one.
		assert.Equal(nms.T(), fmt.Sprintf(`unable to get the "NSTemplateSet" resource for the user in cluster "%s": %s`, memberClusters[1].Name, ErrUnableFetchNSTemplateSet.Error()), err.Error())
	})

	nms.Run(`a "NSTemplateSet" with no provisioned namespaces returns an error`, func() {
		// given
		// Simulate that fetching the user signup, the user space and the
		// "NSTemplateSet" works, but that the latter does not have any
		// provisioned namespaces.
		noProvisionedNamespaces := toolchainv1alpha1.NSTemplateSet{ObjectMeta: metav1.ObjectMeta{Name: "developer-sandbox-nstemplatesset"}, Status: toolchainv1alpha1.NSTemplateSetStatus{ProvisionedNamespaces: make([]toolchainv1alpha1.SpaceNamespace, 0)}}

		fakeClient.MockGet = mockClientGet(userSignup, userSpace, nil, &noProvisionedNamespaces, nil)
		nms.T().Cleanup(func() { fakeClient.MockGet = nil })

		// when
		// Call the function under test.
		err := namespacesManager.ResetNamespaces(ctx)

		// then
		// Assert that the returned error is the expected one.
		assert.Equal(nms.T(), fmt.Sprintf(`the associated NSTemplateSet "%s" in the member cluster "%s" does not have any provisioned namespaces`, noProvisionedNamespaces.Name, memberClusters[1].Name), err.Error())
	})

	nms.Run(`when deleting a namespace, a "not found" error is ignored`, func() {
		// given
		// Simulate that fetching the user signup, user space and
		// NSTemplateSet resources works.
		fakeClient.MockGet = mockClientGet(userSignup, userSpace, nil, userNSTemplateSet, nil)
		nms.T().Cleanup(func() { fakeClient.MockGet = nil })

		// Simulate that a "not found" error is returned when attempting to
		// delete the user's namespaces.
		namespaceName := userNSTemplateSet.Status.ProvisionedNamespaces[0].Name

		notFoundErr := apierrors.NewNotFound(schema.GroupResource{Resource: "namespaces"}, namespaceName)
		fakeClient.MockDelete = func(_ gocontext.Context, _ client.Object, _ ...client.DeleteOption) error {
			return notFoundErr
		}
		nms.T().Cleanup(func() { fakeClient.MockDelete = nil })

		// when
		// Call the function under test.
		err := namespacesManager.ResetNamespaces(ctx)

		// then
		// Assert that no error is returned since "not found" errors are ignored.
		assert.NoError(nms.T(), err)
	})

	nms.Run("when unable to delete a user namespace an error is returned", func() {
		// given
		// Simulate that fetching the user signup, user space and
		// NSTemplateSet resources works.
		fakeClient.MockGet = mockClientGet(userSignup, userSpace, nil, userNSTemplateSet, nil)
		nms.T().Cleanup(func() { fakeClient.MockGet = nil })

		// Simulate that an error is returned when attempting to delete the
		// namespaces.
		fakeClient.MockDelete = func(_ gocontext.Context, _ client.Object, _ ...client.DeleteOption) error {
			return ErrKubernetes
		}
		nms.T().Cleanup(func() { fakeClient.MockDelete = nil })

		// when
		// Call the function under test.
		err := namespacesManager.ResetNamespaces(ctx)

		// then
		// Assert that the returned error is the expected one.
		assert.Equal(nms.T(), fmt.Sprintf(`unable to delete user namespace "%s" in cluster "%s": %s`, userNSTemplateSet.Status.ProvisionedNamespaces[0].Name, memberClusters[1].Name, ErrKubernetes.Error()), err.Error())
	})

	nms.Run(`the "reset namespaces" feature works as expected and does not return any errors`, func() {
		// given
		// Simulate that fetching the user pace and NSTemplateSet resources
		// works and count the number of times each object gets fetched.
		getUserSpaceCount := 0
		getNSTemplateSetCount := 0
		fakeClient.MockGet = func(_ gocontext.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
			if space, ok := obj.(*toolchainv1alpha1.Space); ok {
				*space = *userSpace

				getUserSpaceCount++
			} else if nsTemplateSet, ok := obj.(*toolchainv1alpha1.NSTemplateSet); ok {
				*nsTemplateSet = *userNSTemplateSet

				getNSTemplateSetCount++
			}

			return nil
		}
		nms.T().Cleanup(func() { fakeClient.MockGet = nil })

		// Simulate that deletions work and count the number of namespaces
		// that got deleted.
		deletionCount := 0
		fakeClient.MockDelete = func(_ gocontext.Context, _ client.Object, _ ...client.DeleteOption) error {
			deletionCount++

			return nil
		}
		nms.T().Cleanup(func() { fakeClient.MockDelete = nil })

		// when
		// Call the function under test.
		err := namespacesManager.ResetNamespaces(ctx)
		if err != nil {
			nms.Fail(`unexpected error while calling the "ResetNamespaces" function. None expected`, err.Error())
			return
		}

		// then
		// Assert that the counters hold the expected number.
		assert.Equal(nms.T(), 1, getUserSpaceCount)
		assert.Equal(nms.T(), 1, getNSTemplateSetCount)
		assert.Equal(nms.T(), 3, deletionCount)
	})
}
