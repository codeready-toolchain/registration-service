package auth_test

import (
	"log"
	"os"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/auth"
	testutils "github.com/codeready-toolchain/registration-service/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)
type TestDefaultManagerSuite struct {
	testutils.UnitTestSuite
}

 func TestRunDefaultManagerSuite(t *testing.T) {
	suite.Run(t, &TestDefaultManagerSuite{testutils.UnitTestSuite{}})
}

func (s *TestDefaultManagerSuite) TestKeyManagerDefaultKeyManagerCreation() {
	// Create logger and registry.
	logger := log.New(os.Stderr, "", 0)

	// Set the config for testing mode, the handler may use this.
	assert.True(s.T(), s.Config.IsTestingMode(), "testing mode not set correctly to true")

	s.Run("first creation", func() {
		_, err := auth.InitializeDefaultKeyManager(logger, s.Config)
		require.NoError(s.T(), err)
	})

	s.Run("second redundant creation", func() {
		_, err := auth.InitializeDefaultKeyManager(logger, s.Config)
		require.Error(s.T(), err)
		require.Equal(s.T(), "default KeyManager can be created only once", err.Error())
	})
}
