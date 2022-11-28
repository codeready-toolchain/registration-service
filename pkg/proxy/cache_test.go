package proxy

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/proxy/namespace"
	"github.com/codeready-toolchain/registration-service/test"
	commontest "github.com/codeready-toolchain/toolchain-common/pkg/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	authenticationv1 "k8s.io/api/authentication/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TestCacheSuite struct {
	test.UnitTestSuite
}

func TestRunCacheSuite(t *testing.T) {
	suite.Run(t, &TestCacheSuite{test.UnitTestSuite{}})
}

func (s *TestCacheSuite) TestCache() {

	// given
	memberURL, err := url.Parse("https://my.domain.com")
	require.NoError(s.T(), err)
	fakeApp := &fakeApp{}
	csh := NewUserClusters(fakeApp)

	cl := commontest.NewFakeClient(s.T())
	johnNamespaceAccess := namespace.NewClusterAccess(*memberURL, cl, "someToken", "john")

	s.Run("first time - not found in cache", func() {
		// when
		fakeApp.namespaces = map[string]*namespace.ClusterAccess{
			"johnUserID": johnNamespaceAccess,
		}

		// then
		ns, err := csh.Get(nil, "johnUserID", "john")
		require.NoError(s.T(), err)
		assert.Same(s.T(), johnNamespaceAccess, ns)
	})

	s.Run("second time - valid namespace access found in cache", func() {
		// when
		fakeApp.namespaces = map[string]*namespace.ClusterAccess{} // Fake namespace access service doesn't have any namespace access. To ensure that cache is used instead.
		cl.MockCreate = s.tokenReview(cl, true, nil, "someToken")  // Cached namespace access is valid

		// then
		ns, err := csh.Get(nil, "johnUserID", "john")
		require.NoError(s.T(), err)
		assert.Same(s.T(), johnNamespaceAccess, ns)
	})

	s.Run("third time - found in cache but not valid anymore", func() {
		// when
		updatedJohnNamespaceAccess := namespace.NewClusterAccess(*memberURL, cl, "updatedToken", "john")
		fakeApp.namespaces = map[string]*namespace.ClusterAccess{
			"johnUserID": updatedJohnNamespaceAccess,
		}
		cl.MockCreate = s.tokenReview(cl, false, nil, "someToken") // Cached namespace access is not valid anymore

		// then
		ns, err := csh.Get(nil, "johnUserID", "john")
		require.NoError(s.T(), err)
		assert.Same(s.T(), updatedJohnNamespaceAccess, ns)
	})

	s.Run("fourth time - validation fails with error", func() {
		// when
		cl.MockCreate = s.tokenReview(cl, false, errors.New("unable to create"), "updatedToken")

		// then
		_, err := csh.Get(nil, "johnUserID", "john")
		require.EqualError(s.T(), err, "unable to create")
	})
}

// tokenReview returns a function for the fake client to mock the token review request
func (s *TestCacheSuite) tokenReview(cl client.Writer, authenticated bool, returnError error, expectedTokenToValidate string) func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	return func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
		tr, ok := obj.(*authenticationv1.TokenReview)
		if ok {
			assert.Equal(s.T(), expectedTokenToValidate, tr.Spec.Token)
			tr.Status.Authenticated = authenticated
			return returnError
		}
		return cl.Create(ctx, obj, opts...)
	}
}
