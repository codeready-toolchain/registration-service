package proxy

import (
	"net/url"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/proxy/access"
	"github.com/codeready-toolchain/registration-service/test"
	commontest "github.com/codeready-toolchain/toolchain-common/pkg/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
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
	csh := NewUserAccess(fakeApp)

	cl := commontest.NewFakeClient(s.T())
	johnNamespaceAccess := access.NewClusterAccess(*memberURL, cl, "someToken", "john")

	s.Run("first time - not found in existing cache", func() {
		// when
		fakeApp.accesses = map[string]*access.ClusterAccess{
			"johnUserID": johnNamespaceAccess,
		}

		// then
		ca, err := csh.Get(nil, "johnUserID", "john")
		require.NoError(s.T(), err)
		assert.Same(s.T(), johnNamespaceAccess, ca)
	})

	s.Run("second time - valid cluster access found in cache", func() {
		// when
		fakeApp.accesses = map[string]*access.ClusterAccess{} // Fake access service doesn't have any access. To ensure that cache is used instead.

		// then
		ns, err := csh.Get(nil, "johnUserID", "john")
		require.NoError(s.T(), err)
		assert.Same(s.T(), johnNamespaceAccess, ns)
	})
}
