package proxy

import (
	"testing"

	"github.com/codeready-toolchain/registration-service/test"
	"github.com/stretchr/testify/suite"
)

type TestProxySuite struct {
	test.UnitTestSuite
}

func TestRunProxySuite(t *testing.T) {
	suite.Run(t, &TestProxySuite{test.UnitTestSuite{}})
}

func (s *TestProxySuite) TestProxy() {
	// given

	s.Run("proxy", func() {
	})
}
