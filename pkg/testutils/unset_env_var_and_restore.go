package testutils

import "os"

// UnsetEnvVarAndRestore unsets the given environment variable with the key (if
// present). It returns a function to be called whenever you want to restore the
// original environment.
//
// In a test you can use this to temporarily set an environment variable:
//
//    func TestFoo(t *testing.T) {
//        restoreFunc := testutils.UnsetEnvVarAndRestore("foo")
//        defer restoreFunc()
//        os.Setenv(key, "bar")
//
//        // continue as if foo=bar
//    }
func UnsetEnvVarAndRestore(key string) func() {
	realEnvValue, present := os.LookupEnv(key)
	os.Unsetenv(key)
	return func() {
		if present {
			os.Setenv(key, realEnvValue)
		} else {
			os.Unsetenv(key)
		}
	}
}
