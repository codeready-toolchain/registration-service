package configuration_test

import (
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	testutils "github.com/codeready-toolchain/registration-service/test"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getDefaultConfiguration returns a configuration registry without anything but
// defaults set. Remember that environment variables can overwrite defaults, so
// please ensure to properly unset envionment variables using
// UnsetEnvVarAndRestore().
func getDefaultConfiguration(t *testing.T) *configuration.Registry {
	config, err := configuration.New("")
	require.NoError(t, err)
	return config
}

// getFileConfiguration returns a configuration based on defaults, the given
// file content and overwrites by environment variables. As with
// getDefaultConfiguration() remember that environment variables can overwrite
// defaults, so please ensure to properly unset envionment variables using
// UnsetEnvVarAndRestore().
func getFileConfiguration(t *testing.T, content string) *configuration.Registry {
	tmpFile, err := ioutil.TempFile(os.TempDir(), "configFile-")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	_, err = tmpFile.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())
	config, err := configuration.New(tmpFile.Name())
	require.NoError(t, err)
	return config
}

func TestNew(t *testing.T) {
	t.Run("default configuration", func(t *testing.T) {
		reg, err := configuration.New("")
		require.NoError(t, err)
		require.NotNil(t, reg)
	})
	t.Run("non existing file path", func(t *testing.T) {
		u, err := uuid.NewV4()
		require.NoError(t, err)
		reg, err := configuration.New(u.String())
		require.Error(t, err)
		require.Nil(t, reg)
	})
}

func TestGetHTTPAddress(t *testing.T) {
	key := configuration.EnvPrefix + "_" + "HTTP_ADDRESS"

	t.Run("default", func(t *testing.T) {
		resetFunc := testutils.UnsetEnvVarAndRestore(key)
		defer resetFunc()
		config := getDefaultConfiguration(t)
		assert.Equal(t, configuration.DefaultHTTPAddress, config.GetHTTPAddress())
	})

	t.Run("file", func(t *testing.T) {
		resetFunc := testutils.UnsetEnvVarAndRestore(key)
		defer resetFunc()
		u, err := uuid.NewV4()
		require.NoError(t, err)
		newVal := u.String()
		config := getFileConfiguration(t, `http.address: "`+newVal+`"`)
		assert.Equal(t, newVal, config.GetHTTPAddress())
	})

	t.Run("env overwrite", func(t *testing.T) {
		u, err := uuid.NewV4()
		require.NoError(t, err)
		newVal := u.String()
		os.Setenv(key, newVal)
		config := getDefaultConfiguration(t)
		assert.Equal(t, newVal, config.GetHTTPAddress())
	})
}

func TestGetLogLevel(t *testing.T) {
	key := configuration.EnvPrefix + "_" + "LOG_LEVEL"
	resetFunc := testutils.UnsetEnvVarAndRestore(key)
	defer resetFunc()

	t.Run("default", func(t *testing.T) {
		resetFunc := testutils.UnsetEnvVarAndRestore(key)
		defer resetFunc()
		config := getDefaultConfiguration(t)
		assert.Equal(t, configuration.DefaultLogLevel, config.GetLogLevel())
	})

	t.Run("file", func(t *testing.T) {
		resetFunc := testutils.UnsetEnvVarAndRestore(key)
		defer resetFunc()
		u, err := uuid.NewV4()
		require.NoError(t, err)
		newVal := u.String()
		config := getFileConfiguration(t, `log.level: "`+newVal+`"`)
		assert.Equal(t, newVal, config.GetLogLevel())
	})

	t.Run("env overwrite", func(t *testing.T) {
		u, err := uuid.NewV4()
		require.NoError(t, err)
		newVal := u.String()
		os.Setenv(key, newVal)
		config := getDefaultConfiguration(t)
		assert.Equal(t, newVal, config.GetLogLevel())
	})
}

func TestIsLogJSON(t *testing.T) {
	key := configuration.EnvPrefix + "_" + "LOG_JSON"
	resetFunc := testutils.UnsetEnvVarAndRestore(key)
	defer resetFunc()

	t.Run("default", func(t *testing.T) {
		resetFunc := testutils.UnsetEnvVarAndRestore(key)
		defer resetFunc()
		config := getDefaultConfiguration(t)
		assert.Equal(t, configuration.DefaultLogJSON, config.IsLogJSON())
	})

	t.Run("file", func(t *testing.T) {
		resetFunc := testutils.UnsetEnvVarAndRestore(key)
		defer resetFunc()
		newVal := !configuration.DefaultLogJSON
		config := getFileConfiguration(t, `log.json: "`+strconv.FormatBool(newVal)+`"`)
		assert.Equal(t, newVal, config.IsLogJSON())
	})

	t.Run("env overwrite", func(t *testing.T) {
		newVal := !configuration.DefaultLogJSON
		os.Setenv(key, strconv.FormatBool(newVal))
		config := getDefaultConfiguration(t)
		assert.Equal(t, newVal, config.IsLogJSON())
	})
}

func TestGetGracefulTimeout(t *testing.T) {
	key := configuration.EnvPrefix + "_" + "GRACEFUL_TIMEOUT"

	t.Run("default", func(t *testing.T) {
		resetFunc := testutils.UnsetEnvVarAndRestore(key)
		defer resetFunc()
		config := getDefaultConfiguration(t)
		assert.Equal(t, configuration.DefaultGracefulTimeout, config.GetGracefulTimeout())
	})

	t.Run("file", func(t *testing.T) {
		resetFunc := testutils.UnsetEnvVarAndRestore(key)
		defer resetFunc()
		newVal := 333 * time.Second
		config := getFileConfiguration(t, `graceful_timeout: "`+newVal.String()+`"`)
		assert.Equal(t, newVal, config.GetGracefulTimeout())
	})

	t.Run("env overwrite", func(t *testing.T) {
		newVal := 666 * time.Second
		os.Setenv(key, newVal.String())
		config := getDefaultConfiguration(t)
		assert.Equal(t, newVal, config.GetGracefulTimeout())
	})
}

func TestGetHTTPWriteTimeout(t *testing.T) {
	key := configuration.EnvPrefix + "_" + "HTTP_WRITE_TIMEOUT"

	t.Run("default", func(t *testing.T) {
		resetFunc := testutils.UnsetEnvVarAndRestore(key)
		defer resetFunc()
		config := getDefaultConfiguration(t)
		assert.Equal(t, configuration.DefaultHTTPWriteTimeout, config.GetHTTPWriteTimeout())
	})

	t.Run("file", func(t *testing.T) {
		resetFunc := testutils.UnsetEnvVarAndRestore(key)
		defer resetFunc()
		newVal := 333 * time.Second
		config := getFileConfiguration(t, `http.write_timeout: "`+newVal.String()+`"`)
		assert.Equal(t, newVal, config.GetHTTPWriteTimeout())
	})

	t.Run("env overwrite", func(t *testing.T) {
		newVal := 666 * time.Second
		os.Setenv(key, newVal.String())
		config := getDefaultConfiguration(t)
		assert.Equal(t, newVal, config.GetHTTPWriteTimeout())
	})
}

func TestGetHTTPReadTimeout(t *testing.T) {
	key := configuration.EnvPrefix + "_" + "HTTP_READ_TIMEOUT"

	t.Run("default", func(t *testing.T) {
		resetFunc := testutils.UnsetEnvVarAndRestore(key)
		defer resetFunc()
		config := getDefaultConfiguration(t)
		assert.Equal(t, configuration.DefaultHTTPReadTimeout, config.GetHTTPReadTimeout())
	})

	t.Run("file", func(t *testing.T) {
		resetFunc := testutils.UnsetEnvVarAndRestore(key)
		defer resetFunc()
		newVal := 444 * time.Second
		config := getFileConfiguration(t, `http.read_timeout: "`+newVal.String()+`"`)
		assert.Equal(t, newVal, config.GetHTTPReadTimeout())
	})

	t.Run("env overwrite", func(t *testing.T) {
		newVal := 777 * time.Second
		os.Setenv(key, newVal.String())
		config := getDefaultConfiguration(t)
		assert.Equal(t, newVal, config.GetHTTPReadTimeout())
	})
}

func TestGetHTTPIdleTimeout(t *testing.T) {
	key := configuration.EnvPrefix + "_" + "HTTP_IDLE_TIMEOUT"

	t.Run("default", func(t *testing.T) {
		resetFunc := testutils.UnsetEnvVarAndRestore(key)
		defer resetFunc()
		config := getDefaultConfiguration(t)
		assert.Equal(t, configuration.DefaultHTTPIdleTimeout, config.GetHTTPIdleTimeout())
	})

	t.Run("file", func(t *testing.T) {
		resetFunc := testutils.UnsetEnvVarAndRestore(key)
		defer resetFunc()
		newVal := 111 * time.Second
		config := getFileConfiguration(t, `http.idle_timeout: "`+newVal.String()+`"`)
		assert.Equal(t, newVal, config.GetHTTPIdleTimeout())
	})

	t.Run("env overwrite", func(t *testing.T) {
		newVal := 888 * time.Second
		os.Setenv(key, newVal.String())
		config := getDefaultConfiguration(t)
		assert.Equal(t, newVal, config.GetHTTPIdleTimeout())
	})
}

func TestGetHTTPCompressResponses(t *testing.T) {
	key := configuration.EnvPrefix + "_" + "HTTP_COMPRESS"

	t.Run("default", func(t *testing.T) {
		resetFunc := testutils.UnsetEnvVarAndRestore(key)
		defer resetFunc()
		config := getDefaultConfiguration(t)
		assert.Equal(t, configuration.DefaultHTTPCompressResponses, config.GetHTTPCompressResponses())
	})

	t.Run("file", func(t *testing.T) {
		resetFunc := testutils.UnsetEnvVarAndRestore(key)
		defer resetFunc()
		newVal := !configuration.DefaultHTTPCompressResponses
		config := getFileConfiguration(t, `http.compress: "`+strconv.FormatBool(newVal)+`"`)
		assert.Equal(t, newVal, config.GetHTTPCompressResponses())
	})

	t.Run("env overwrite", func(t *testing.T) {
		newVal := !configuration.DefaultHTTPCompressResponses
		os.Setenv(key, strconv.FormatBool(newVal))
		config := getDefaultConfiguration(t)
		assert.Equal(t, newVal, config.GetHTTPCompressResponses())
	})
}

func TestIsTestingMode(t *testing.T) {
	key := configuration.EnvPrefix + "_" + "TESTINGMODE"
	resetFunc := testutils.UnsetEnvVarAndRestore(key)
	defer resetFunc()

	t.Run("default", func(t *testing.T) {
		resetFunc := testutils.UnsetEnvVarAndRestore(key)
		defer resetFunc()
		config := getDefaultConfiguration(t)
		assert.Equal(t, configuration.DefaultTestingMode, config.IsTestingMode())
	})

	t.Run("file", func(t *testing.T) {
		resetFunc := testutils.UnsetEnvVarAndRestore(key)
		defer resetFunc()
		newVal := !configuration.DefaultTestingMode
		config := getFileConfiguration(t, `testingmode: "`+strconv.FormatBool(newVal)+`"`)
		assert.Equal(t, newVal, config.IsTestingMode())
	})

	t.Run("env overwrite", func(t *testing.T) {
		newVal := !configuration.DefaultTestingMode
		os.Setenv(key, strconv.FormatBool(newVal))
		config := getDefaultConfiguration(t)
		assert.Equal(t, newVal, config.IsTestingMode())
	})
}

func TestGetAuthClientConfig(t *testing.T) {
	key := configuration.EnvPrefix + "_" + "AUTH_CLIENT_CONFIG_AUTH-SERVER-URL"

	t.Run("default", func(t *testing.T) {
		resetFunc := testutils.UnsetEnvVarAndRestore(key)
		defer resetFunc()
		config := getDefaultConfiguration(t)
		assert.Equal(t, configuration.DefaultAuthClientConfigAuthServerURL, config.GetAuthClientConfigAuthServerURL())
	})

	t.Run("file", func(t *testing.T) {
		resetFunc := testutils.UnsetEnvVarAndRestore(key)
		defer resetFunc()
		u, err := uuid.NewV4()
		require.NoError(t, err)
		newVal := u.String()
		config := getFileConfiguration(t, `auth_client.config.auth-server-url: "`+newVal+`"`)
		assert.Equal(t, newVal, config.GetAuthClientConfigAuthServerURL())
	})

	t.Run("env overwrite", func(t *testing.T) {
		u, err := uuid.NewV4()
		require.NoError(t, err)
		newVal := u.String()
		os.Setenv(key, newVal)
		config := getDefaultConfiguration(t)
		assert.Equal(t, newVal, config.GetAuthClientConfigAuthServerURL())
	})
}

func TestGetAuthClientConfigRealm(t *testing.T) {
	key := configuration.EnvPrefix + "_" + "AUTH_CLIENT_CONFIG_REALM"

	t.Run("default", func(t *testing.T) {
		resetFunc := testutils.UnsetEnvVarAndRestore(key)
		defer resetFunc()
		config := getDefaultConfiguration(t)
		assert.Equal(t, configuration.DefaultAuthClientConfigRealm, config.GetAuthClientConfigRealm())
	})

	t.Run("file", func(t *testing.T) {
		resetFunc := testutils.UnsetEnvVarAndRestore(key)
		defer resetFunc()
		u, err := uuid.NewV4()
		require.NoError(t, err)
		newVal := u.String()
		config := getFileConfiguration(t, `auth_client.config.realm: "`+newVal+`"`)
		assert.Equal(t, newVal, config.GetAuthClientConfigRealm())
	})

	t.Run("env overwrite", func(t *testing.T) {
		u, err := uuid.NewV4()
		require.NoError(t, err)
		newVal := u.String()
		os.Setenv(key, newVal)
		config := getDefaultConfiguration(t)
		assert.Equal(t, newVal, config.GetAuthClientConfigRealm())
	})
}

func TestGetAuthClientConfigSSLRequired(t *testing.T) {
	key := configuration.EnvPrefix + "_" + "AUTH_CLIENT_CONFIG_SSL-REQUIRED"

	t.Run("default", func(t *testing.T) {
		resetFunc := testutils.UnsetEnvVarAndRestore(key)
		defer resetFunc()
		config := getDefaultConfiguration(t)
		assert.Equal(t, configuration.DefaultAuthClientConfigSSLRequired, config.GetAuthClientConfigSSLRequired())
	})

	t.Run("file", func(t *testing.T) {
		resetFunc := testutils.UnsetEnvVarAndRestore(key)
		defer resetFunc()
		u, err := uuid.NewV4()
		require.NoError(t, err)
		newVal := u.String()
		config := getFileConfiguration(t, `auth_client.config.ssl-required: "`+newVal+`"`)
		assert.Equal(t, newVal, config.GetAuthClientConfigSSLRequired())
	})

	t.Run("env overwrite", func(t *testing.T) {
		u, err := uuid.NewV4()
		require.NoError(t, err)
		newVal := u.String()
		os.Setenv(key, newVal)
		config := getDefaultConfiguration(t)
		assert.Equal(t, newVal, config.GetAuthClientConfigSSLRequired())
	})
}

func TestGetAuthClientConfigResource(t *testing.T) {
	key := configuration.EnvPrefix + "_" + "AUTH_CLIENT_CONFIG_RESOURCE"

	t.Run("default", func(t *testing.T) {
		resetFunc := testutils.UnsetEnvVarAndRestore(key)
		defer resetFunc()
		config := getDefaultConfiguration(t)
		assert.Equal(t, configuration.DefaultAuthClientConfigResource, config.GetAuthClientConfigResource())
	})

	t.Run("file", func(t *testing.T) {
		resetFunc := testutils.UnsetEnvVarAndRestore(key)
		defer resetFunc()
		u, err := uuid.NewV4()
		require.NoError(t, err)
		newVal := u.String()
		config := getFileConfiguration(t, `auth_client.config.resource: "`+newVal+`"`)
		assert.Equal(t, newVal, config.GetAuthClientConfigResource())
	})

	t.Run("env overwrite", func(t *testing.T) {
		u, err := uuid.NewV4()
		require.NoError(t, err)
		newVal := u.String()
		os.Setenv(key, newVal)
		config := getDefaultConfiguration(t)
		assert.Equal(t, newVal, config.GetAuthClientConfigResource())
	})
}

func TestGetAuthClientConfigPublicResource(t *testing.T) {
	key := configuration.EnvPrefix + "_" + "AUTH_CLIENT_CONFIG_PUBLIC-CLIENT"
	resetFunc := testutils.UnsetEnvVarAndRestore(key)
	defer resetFunc()

	t.Run("default", func(t *testing.T) {
		resetFunc := testutils.UnsetEnvVarAndRestore(key)
		defer resetFunc()
		config := getDefaultConfiguration(t)
		assert.Equal(t, configuration.DefaultAuthClientConfigPublicClient, config.IsAuthClientConfigPublicClient())
	})

	t.Run("file", func(t *testing.T) {
		resetFunc := testutils.UnsetEnvVarAndRestore(key)
		defer resetFunc()
		newVal := !configuration.DefaultAuthClientConfigPublicClient
		config := getFileConfiguration(t, `auth_client.config.public-client: "`+strconv.FormatBool(newVal)+`"`)
		assert.Equal(t, newVal, config.IsAuthClientConfigPublicClient())
	})

	t.Run("env overwrite", func(t *testing.T) {
		newVal := !configuration.DefaultAuthClientConfigPublicClient
		os.Setenv(key, strconv.FormatBool(newVal))
		config := getDefaultConfiguration(t)
		assert.Equal(t, newVal, config.IsAuthClientConfigPublicClient())
	})
}

func TestGetAuthClientConfigConfidentialPort(t *testing.T) {
	key := configuration.EnvPrefix + "_" + "AUTH_CLIENT_CONFIG_CONFIDENTIAL-PORT"
	resetFunc := testutils.UnsetEnvVarAndRestore(key)
	defer resetFunc()

	t.Run("default", func(t *testing.T) {
		resetFunc := testutils.UnsetEnvVarAndRestore(key)
		defer resetFunc()
		config := getDefaultConfiguration(t)
		assert.Equal(t, configuration.DefaultAuthClientConfigConfidentialPort, config.GetAuthClientConfigConfidentialPort())
	})

	t.Run("file", func(t *testing.T) {
		resetFunc := testutils.UnsetEnvVarAndRestore(key)
		defer resetFunc()
		newVal := rand.Int63n(100)
		config := getFileConfiguration(t, `auth_client.config.confidential-port: "`+strconv.FormatInt(newVal, 10)+`"`)
		assert.Equal(t, newVal, config.GetAuthClientConfigConfidentialPort())
	})

	t.Run("env overwrite", func(t *testing.T) {
		newVal := rand.Int63n(100)
		os.Setenv(key, strconv.FormatInt(newVal, 10))
		config := getDefaultConfiguration(t)
		assert.Equal(t, newVal, config.GetAuthClientConfigConfidentialPort())
	})

}
