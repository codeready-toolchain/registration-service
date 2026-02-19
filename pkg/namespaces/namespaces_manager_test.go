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

// createMockMemberClusters creates a slice of three member clusters with the
// given client associated with them.
func createMockMemberClusters(nsClient namespaced.Client) []*cluster.CachedToolchainCluster {
	memberClusters := make([]*cluster.CachedToolchainCluster, 0)

	memberClusters = append(memberClusters, createNewMemberCluster(nsClient, "member-cluster-1"))
	memberClusters = append(memberClusters, createNewMemberCluster(nsClient, "member-cluster-2"))
	memberClusters = append(memberClusters, createNewMemberCluster(nsClient, "member-cluster-3"))

	return memberClusters
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

// TestNamespacesManagerSuite holds the unit test suite to be able to run the
// tests and the test fixtures that will be used throughout the tests.
type TestNamespacesManagerSuite struct {
	test.UnitTestSuite
}

func TestRunNamespacesManagerSuite(t *testing.T) {
	suite.Run(t, &TestNamespacesManagerSuite{test.UnitTestSuite{}})
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

	// Create all the standard fixtures for the tests.
	// mockSignup is the mocked signup returned by the registration service.
	mockSignup := []*signup.Signup{
		{
			Name:              TestUsername,
			Username:          TestUsername,
			CompliantUsername: TestUsername,
			Status: signup.Status{
				Ready: true,
			},
		},
	}

	// mockSpace represents the mocked space in which the user will supposedly
	// have their namespaces set up.
	mockSpace := &toolchainv1alpha1.Space{Spec: toolchainv1alpha1.SpaceSpec{TargetCluster: "member-cluster-2"}}

	// mockNSTemplateSet represents the mocked NSTemplateSet associated with
	// the user, which holds the namespaces that they have provisioned.
	mockNSTemplateSet := &toolchainv1alpha1.NSTemplateSet{Status: toolchainv1alpha1.NSTemplateSetStatus{ProvisionedNamespaces: []toolchainv1alpha1.SpaceNamespace{}}}
	mockNSTemplateSet.Status.ProvisionedNamespaces = append(mockNSTemplateSet.Status.ProvisionedNamespaces, toolchainv1alpha1.SpaceNamespace{Name: "namespace-1"})
	mockNSTemplateSet.Status.ProvisionedNamespaces = append(mockNSTemplateSet.Status.ProvisionedNamespaces, toolchainv1alpha1.SpaceNamespace{Name: "namespace-2"})
	mockNSTemplateSet.Status.ProvisionedNamespaces = append(mockNSTemplateSet.Status.ProvisionedNamespaces, toolchainv1alpha1.SpaceNamespace{Name: "namespace-3"})

	nms.Run("the user signup service returns an error", func() {
		// given
		fakeClient := commontest.NewFakeClient(nms.T())
		nsClient := namespaced.NewClient(fakeClient, "namespace")

		// Simulate an error when attempting to fetch the user's signup
		// resource.
		fakeSignupService := fake.NewSignupService(mockSignup...)
		fakeSignupService.MockGetSignup = func(_ string) (*signup.Signup, error) {
			return nil, ErrUnableFetchUserSignup
		}

		namespacesManager := NewNamespacesManager(getMemberClusters(make([]*cluster.CachedToolchainCluster, 0)), nsClient, fakeSignupService)

		// when
		// Call the function under test.
		err := namespacesManager.ResetNamespaces(ctx)

		// then
		// Assert that the returned error is the expected one.
		assert.Equal(nms.T(), fmt.Sprintf("unable to obtain the user signup: %s", ErrUnableFetchUserSignup.Error()), err.Error())
	})

	nms.Run(`the signup service returns a "not found or deactivated" response`, func() {
		// given
		fakeClient := commontest.NewFakeClient(nms.T())
		nsClient := namespaced.NewClient(fakeClient, "namespace")

		// Simulate that the user service returns a "not found" or "inactive"
		// response by first returning a "nil" user signup, and then some user
		// signups with blank compliant usernames.
		fakeSignupService := fake.NewSignupService(mockSignup...)

		namespacesManager := NewNamespacesManager(getMemberClusters(make([]*cluster.CachedToolchainCluster, 0)), nsClient, fakeSignupService)

		testCases := []struct {
			signup *signup.Signup
		}{
			{signup: nil},
			{signup: &signup.Signup{CompliantUsername: ""}},
			{signup: &signup.Signup{CompliantUsername: "     "}},
		}

		for _, testCase := range testCases {
			fakeSignupService.MockGetSignup = func(_ string) (*signup.Signup, error) {
				return testCase.signup, nil
			}

			// when
			// Call the function under test.
			err := namespacesManager.ResetNamespaces(ctx)

			// then
			// Assert that the returned error is the expected one.
			assert.ErrorIs(nms.T(), err, ErrUserSignUpNotFoundOrDeactivated)
		}
	})

	nms.Run("fetching user space returns error", func() {
		// given
		// Simulate that there is an error when attempting to fetch the user's
		// space.
		fakeClient := commontest.NewFakeClient(nms.T())
		fakeClient.MockGet = func(_ gocontext.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
			return ErrUnableFetchSpace
		}

		nsClient := namespaced.NewClient(fakeClient, "namespace")
		fakeSignupService := fake.NewSignupService(mockSignup...)
		namespacesManager := NewNamespacesManager(getMemberClusters(make([]*cluster.CachedToolchainCluster, 0)), nsClient, fakeSignupService)

		// when
		// Call the function under test.
		err := namespacesManager.ResetNamespaces(ctx)

		// then
		// Assert that the returned error is the expected one.
		assert.Equal(nms.T(), fmt.Sprintf("unable to get user's space resource: %s", ErrUnableFetchSpace.Error()), err.Error())
	})

	nms.Run("not being able to locate the cluster the user has been provisioned in returns error", func() {
		// given
		// Simulate that the call to fetch the user space resource is
		// successful, but that it returns a cluster that will not generate a
		// match.
		fakeClient := commontest.NewFakeClient(nms.T())
		inexistentClusterNameSpace := &toolchainv1alpha1.Space{
			Spec: toolchainv1alpha1.SpaceSpec{
				TargetCluster: "inexistent-cluster",
			},
		}
		fakeClient.MockGet = func(_ gocontext.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
			if space, ok := obj.(*toolchainv1alpha1.Space); ok {
				*space = *inexistentClusterNameSpace
				return nil
			}

			return ErrKubernetes
		}

		nsClient := namespaced.NewClient(fakeClient, "namespace")
		fakeSignupService := fake.NewSignupService(mockSignup...)
		namespacesManager := NewNamespacesManager(getMemberClusters(make([]*cluster.CachedToolchainCluster, 0)), nsClient, fakeSignupService)

		// when
		// Call the function under test.
		err := namespacesManager.ResetNamespaces(ctx)

		// then
		// Assert that the returned error is the expected one.
		assert.Equal(nms.T(), fmt.Sprintf(`unable to locate the target cluster "%s" for the user`, inexistentClusterNameSpace.Spec.TargetCluster), err.Error())
	})

	nms.Run("a member cluster without a client will return an error", func() {
		// given
		// Simulate that the user's space is fetched without any errors.
		fakeClient := commontest.NewFakeClient(nms.T())
		fakeClient.MockGet = func(_ gocontext.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
			if space, ok := obj.(*toolchainv1alpha1.Space); ok {
				*space = *mockSpace
				return nil
			}

			return ErrKubernetes
		}

		nsClient := namespaced.NewClient(fakeClient, "namespace")
		fakeSignupService := fake.NewSignupService(mockSignup...)

		mockMemberClusters := createMockMemberClusters(nsClient)
		namespacesManager := NewNamespacesManager(getMemberClusters(mockMemberClusters), nsClient, fakeSignupService)

		// Simulate that the user's target cluster does not have a client.
		mockMemberClusters[1].Client = nil

		// when
		// Call the function under test.
		err := namespacesManager.ResetNamespaces(ctx)

		// then
		// Assert that the returned error is the expected one.
		assert.Equal(nms.T(), fmt.Sprintf(`unable to obtain the client for cluster "%s"`, mockMemberClusters[1].Name), err.Error())
	})

	nms.Run("not being able to fetch the user's NSTemplateSet will return an error", func() {
		// given
		// Simulate that fetching the user space works, but that fetching the
		// "NSTemplateSet" gives an error.
		fakeClient := commontest.NewFakeClient(nms.T())
		fakeClient.MockGet = func(_ gocontext.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
			if space, ok := obj.(*toolchainv1alpha1.Space); ok {
				*space = *mockSpace
				return nil
			}

			return ErrUnableFetchNSTemplateSet
		}
		nsClient := namespaced.NewClient(fakeClient, "namespace")
		fakeSignupService := fake.NewSignupService(mockSignup...)

		mockMemberClusters := createMockMemberClusters(nsClient)
		namespacesManager := NewNamespacesManager(getMemberClusters(mockMemberClusters), nsClient, fakeSignupService)

		// when
		// Call the function under test.
		err := namespacesManager.ResetNamespaces(ctx)

		// then
		// Assert that the returned error is the expected one.
		assert.Equal(nms.T(), fmt.Sprintf(`unable to get the "NSTemplateSet" resource for the user in cluster "%s": %s`, mockMemberClusters[1].Name, ErrUnableFetchNSTemplateSet.Error()), err.Error())
	})

	nms.Run(`a "NSTemplateSet" with no provisioned namespaces returns an error`, func() {
		// given
		// Simulate that fetching the user space and the "NSTemplateSet"
		// works, but that the latter does not have any provisioned
		// namespaces.
		noProvisionedNamespaces := &toolchainv1alpha1.NSTemplateSet{ObjectMeta: metav1.ObjectMeta{Name: "developer-sandbox-nstemplatesset"}, Status: toolchainv1alpha1.NSTemplateSetStatus{ProvisionedNamespaces: make([]toolchainv1alpha1.SpaceNamespace, 0)}}

		fakeClient := commontest.NewFakeClient(nms.T())
		fakeClient.MockGet = func(_ gocontext.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
			if space, ok := obj.(*toolchainv1alpha1.Space); ok {
				*space = *mockSpace
				return nil
			} else if nsTemplateSet, ok := obj.(*toolchainv1alpha1.NSTemplateSet); ok {
				*nsTemplateSet = *noProvisionedNamespaces
				return nil
			}

			return ErrKubernetes
		}

		nsClient := namespaced.NewClient(fakeClient, "namespace")
		fakeSignupService := fake.NewSignupService(mockSignup...)

		mockMemberClusters := createMockMemberClusters(nsClient)
		namespacesManager := NewNamespacesManager(getMemberClusters(mockMemberClusters), nsClient, fakeSignupService)

		// when
		// Call the function under test.
		err := namespacesManager.ResetNamespaces(ctx)

		// then
		// Assert that the returned error is the expected one.
		assert.Equal(nms.T(), fmt.Sprintf(`the associated NSTemplateSet "%s" in the member cluster "%s" does not have any provisioned namespaces`, noProvisionedNamespaces.Name, mockMemberClusters[1].Name), err.Error())
	})

	nms.Run(`when deleting a namespace, a "not found" error is ignored`, func() {
		// given
		// Simulate that fetching the user space and "NSTemplateSet" resources
		// works.
		fakeClient := commontest.NewFakeClient(nms.T())
		fakeClient.MockGet = func(_ gocontext.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
			if space, ok := obj.(*toolchainv1alpha1.Space); ok {
				*space = *mockSpace
				return nil
			} else if nsTemplateSet, ok := obj.(*toolchainv1alpha1.NSTemplateSet); ok {
				*nsTemplateSet = *mockNSTemplateSet
				return nil
			}

			return ErrKubernetes
		}

		// Simulate that a "not found" error is returned when attempting to
		// delete the user's namespaces.
		namespaceName := mockNSTemplateSet.Status.ProvisionedNamespaces[0].Name

		notFoundErr := apierrors.NewNotFound(schema.GroupResource{Resource: "namespaces"}, namespaceName)
		fakeClient.MockDelete = func(_ gocontext.Context, _ client.Object, _ ...client.DeleteOption) error {
			return notFoundErr
		}

		nsClient := namespaced.NewClient(fakeClient, "namespace")
		fakeSignupService := fake.NewSignupService(mockSignup...)

		mockMemberClusters := createMockMemberClusters(nsClient)
		namespacesManager := NewNamespacesManager(getMemberClusters(mockMemberClusters), nsClient, fakeSignupService)

		// when
		// Call the function under test.
		err := namespacesManager.ResetNamespaces(ctx)

		// then
		// Assert that no error is returned since "not found" errors are ignored.
		assert.NoError(nms.T(), err)
	})

	nms.Run("when unable to delete a user namespace an error is returned", func() {
		// given
		// Simulate that fetching the user space and "NSTemplateSet" resources
		// works.
		fakeClient := commontest.NewFakeClient(nms.T())
		fakeClient.MockGet = func(_ gocontext.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
			if space, ok := obj.(*toolchainv1alpha1.Space); ok {
				*space = *mockSpace
				return nil
			} else if nsTemplateSet, ok := obj.(*toolchainv1alpha1.NSTemplateSet); ok {
				*nsTemplateSet = *mockNSTemplateSet
				return nil
			}

			return ErrKubernetes
		}

		// Simulate that an error is returned when attempting to delete the
		// namespaces.
		fakeClient.MockDelete = func(_ gocontext.Context, _ client.Object, _ ...client.DeleteOption) error {
			return ErrKubernetes
		}

		nsClient := namespaced.NewClient(fakeClient, "namespace")
		fakeSignupService := fake.NewSignupService(mockSignup...)
		mockMemberClusters := createMockMemberClusters(nsClient)
		namespacesManager := NewNamespacesManager(getMemberClusters(mockMemberClusters), nsClient, fakeSignupService)

		// when
		// Call the function under test.
		err := namespacesManager.ResetNamespaces(ctx)

		// then
		// Assert that the returned error is the expected one.
		assert.Equal(nms.T(), fmt.Sprintf(`unable to delete user namespace "%s" in cluster "%s": %s`, mockNSTemplateSet.Status.ProvisionedNamespaces[0].Name, mockMemberClusters[1].Name, ErrKubernetes.Error()), err.Error())
	})

	nms.Run(`the "reset namespaces" feature works as expected and does not return any errors`, func() {
		// given
		// Simulate that fetching the user space and "NSTemplateSet" resources
		// works and count the number of times each object gets fetched.
		getUserSpaceCount := 0
		getNSTemplateSetCount := 0

		fakeClient := commontest.NewFakeClient(nms.T())
		fakeClient.MockGet = func(_ gocontext.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
			if space, ok := obj.(*toolchainv1alpha1.Space); ok {
				*space = *mockSpace

				getUserSpaceCount++
				return nil
			} else if nsTemplateSet, ok := obj.(*toolchainv1alpha1.NSTemplateSet); ok {
				*nsTemplateSet = *mockNSTemplateSet

				getNSTemplateSetCount++
				return nil
			}

			return ErrKubernetes
		}

		// Simulate that deletions work and count the number of namespaces
		// that got deleted.
		deletionCount := 0
		fakeClient.MockDelete = func(_ gocontext.Context, _ client.Object, _ ...client.DeleteOption) error {
			deletionCount++

			return nil
		}

		nsClient := namespaced.NewClient(fakeClient, "namespace")
		fakeSignupService := fake.NewSignupService(mockSignup...)

		mockMemberClusters := createMockMemberClusters(nsClient)
		namespacesManager := NewNamespacesManager(getMemberClusters(mockMemberClusters), nsClient, fakeSignupService)

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
