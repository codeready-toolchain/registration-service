package configuration_test

import (
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/test"
	testconfig "github.com/codeready-toolchain/toolchain-common/pkg/test/config"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestConfigurationSuite struct {
	test.UnitTestSuite
}

func TestRunConfigurationSuite(t *testing.T) {
	suite.Run(t, &TestConfigurationSuite{test.UnitTestSuite{}})
}

func (s *TestConfigurationSuite) TestSegmentWriteKey() {
	s.Run("unit-test environment (default)", func() {
		require.True(s.T(), configuration.IsTestingMode())
	})

	s.Run("prod environment", func() {
		s.SetConfig(testconfig.RegistrationService().Environment(configuration.DefaultEnvironment))
		require.False(s.T(), configuration.IsTestingMode())
	})
}
