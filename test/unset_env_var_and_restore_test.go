package testutils

import (
	"os"
	"testing"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestUnsetEnvVarAndRestoreSuite struct {
	UnitTestSuite
}

 func TestRunUnsetEnvVarAndRestoreSuite(t *testing.T) {
	suite.Run(t, &TestUnsetEnvVarAndRestoreSuite{UnitTestSuite{}})
}

func (s *TestUnsetEnvVarAndRestoreSuite) TestUnsetEnvVarAndRestore() {
	s.Run("check unsetting and restoring of previously unset variable", func() {
		// given
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		varName := u.String()
		os.Unsetenv(varName)
		_, present := os.LookupEnv(varName)
		require.False(s.T(), present)

		// when
		resetFn := UnsetEnvVarAndRestore(varName)

		// then
		_, present = os.LookupEnv(varName)
		require.False(s.T(), present)

		// finally
		resetFn()
		_, present = os.LookupEnv(varName)
		require.False(s.T(), present)
	})

	s.Run("check unsetting and restoring of previously set variable", func() {
		// given
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		varName := u.String()
		val := "somevalue"
		os.Setenv(varName, val)
		_, present := os.LookupEnv(varName)
		require.True(s.T(), present)

		// when
		resetFn := UnsetEnvVarAndRestore(varName)

		// then
		_, present = os.LookupEnv(varName)
		require.False(s.T(), present)

		// finally
		resetFn()
		valAfterRestoring, present := os.LookupEnv(varName)
		require.True(s.T(), present)
		require.Equal(s.T(), val, valAfterRestoring)
	})
}
