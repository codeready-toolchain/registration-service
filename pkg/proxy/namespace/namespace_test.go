package namespace_test

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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TestNamespaceSuite struct {
	test.UnitTestSuite
}

func TestRunNamespaceSuite(t *testing.T) {
	suite.Run(t, &TestNamespaceSuite{test.UnitTestSuite{}})
}

func (s *TestNamespaceSuite) TestValidate() {
	// given
	apiServerURL, err := url.Parse("https://api.domain.com")
	require.NoError(s.T(), err)

	s.Run("client returns error", func() {
		// given
		cl := commontest.NewFakeClient(s.T())
		na := namespace.NewNamespaceAccess(*apiServerURL, "super-secret-token", cl)
		cl.MockCreate = func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
			return errors.New("unable to create")
		}

		valid, err := na.Validate()
		require.EqualError(s.T(), err, "unable to create")

		assert.True(s.T(), valid)
	})

	s.Run("validate", func() {
		// given
		cl := commontest.NewFakeClient(s.T())
		na := namespace.NewNamespaceAccess(*apiServerURL, "super-secret-token", cl)

		valid, err := na.Validate()
		require.NoError(s.T(), err)

		assert.False(s.T(), valid)
	})
}
