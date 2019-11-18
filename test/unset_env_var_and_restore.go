package test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// UnsetEnvVarAndRestore unsets the given environment variable with the key (if
// present). It returns a function to be called whenever you want to restore the
// original environment.
//
// In a test you can use this to temporarily set an environment variable:
//
//    func TestFoo(t *testing.T) {
//        restoreFunc := test.UnsetEnvVarAndRestore(t, "foo")
//        defer restoreFunc()
//        os.Setenv(key, "bar")
//
//        // continue as if foo=bar
//    }
func UnsetEnvVarAndRestore(t *testing.T, key string) func() {
	realEnvValue, present := os.LookupEnv(key)
	err := os.Unsetenv(key)
	require.NoError(t, err)
	return func() {
		if present {
			err := os.Setenv(key, realEnvValue)
			require.NoError(t, err)
		} else {
			err := os.Unsetenv(key)
			require.NoError(t, err)
		}
	}
}

func SetEnvVarAndRestore(t *testing.T, key, newValue string) func() {
	oldEnvValue, present := os.LookupEnv(key)
	err := os.Setenv(key, newValue)
	require.NoError(t, err)
	return func() {
		if present {
			err := os.Setenv(key, oldEnvValue)
			require.NoError(t, err)
		} else {
			err := os.Unsetenv(key)
			require.NoError(t, err)
		}
	}
}
