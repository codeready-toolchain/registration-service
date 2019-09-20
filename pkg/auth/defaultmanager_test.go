package auth_test

import (
	"log"
	"os"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/auth"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeyManagerDefaultKeyManagerCreation(t *testing.T) {
	// Create logger and registry.
	logger := log.New(os.Stderr, "", 0)
	configRegistry := configuration.CreateEmptyRegistry()

	// Set the config for testing mode, the handler may use this.
	configRegistry.GetViperInstance().Set("testingmode", true)
	assert.True(t, configRegistry.IsTestingMode(), "testing mode not set correctly to true")

	t.Run("first creation", func(t *testing.T) {
		_, err := auth.DefaultKeyManagerWithConfig(logger, configRegistry)
		require.NoError(t, err)
	})

	t.Run("second redundant creation", func(t *testing.T) {
		_, err := auth.DefaultKeyManagerWithConfig(logger, configRegistry)
		require.Error(t, err)
		require.Equal(t, "default KeyManager can be created only once", err.Error())
	})
}
