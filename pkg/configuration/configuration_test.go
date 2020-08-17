package configuration_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/test"
	. "github.com/codeready-toolchain/toolchain-common/pkg/test"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestConfigurationSuite struct {
	test.UnitTestSuite
}

func TestRunConfigurationSuite(t *testing.T) {
	suite.Run(t, &TestConfigurationSuite{test.UnitTestSuite{}})
}

// getDefaultConfiguration returns a configuration registry without anything but
// defaults set. Remember that environment variables can overwrite defaults, so
// please ensure to properly unset envionment variables using
// UnsetEnvVarAndRestore().
func (s *TestConfigurationSuite) getDefaultConfiguration() *configuration.Registry {
	config, err := configuration.New("", NewFakeClient(s.T()))
	require.NoError(s.T(), err)
	return config
}

// getFileConfiguration returns a configuration based on defaults, the given
// file content and overwrites by environment variables. As with
// getDefaultConfiguration() remember that environment variables can overwrite
// defaults, so please ensure to properly unset envionment variables using
// UnsetEnvVarAndRestore().
func (s *TestConfigurationSuite) getFileConfiguration(content string) *configuration.Registry {
	tmpFile, err := ioutil.TempFile(os.TempDir(), "configFile-")
	require.NoError(s.T(), err)
	defer func() {
		err := os.Remove(tmpFile.Name())
		require.NoError(s.T(), err)
	}()
	_, err = tmpFile.Write([]byte(content))
	require.NoError(s.T(), err)
	require.NoError(s.T(), tmpFile.Close())
	config, err := configuration.New(tmpFile.Name(), NewFakeClient(s.T()))
	require.NoError(s.T(), err)
	return config
}

func (s *TestConfigurationSuite) TestNew() {
	s.Run("default configuration", func() {
		reg, err := configuration.New("", NewFakeClient(s.T()))
		require.NoError(s.T(), err)
		require.NotNil(s.T(), reg)
	})
	s.Run("non existing file path", func() {
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		reg, err := configuration.New(u.String(), NewFakeClient(s.T()))
		require.Error(s.T(), err)
		require.Nil(s.T(), reg)
	})
}

func (s *TestConfigurationSuite) TestGetHTTPAddress() {
	key := configuration.EnvPrefix + "_" + "HTTP_ADDRESS"

	s.Run("default", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), configuration.DefaultHTTPAddress, config.GetHTTPAddress())
	})

	s.Run("file", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		newVal := u.String()
		config := s.getFileConfiguration(`http.address: "` + newVal + `"`)
		assert.Equal(s.T(), newVal, config.GetHTTPAddress())
	})

	s.Run("env overwrite", func() {
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		newVal := u.String()
		err = os.Setenv(key, newVal)
		require.NoError(s.T(), err)
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), newVal, config.GetHTTPAddress())
	})
}

func (s *TestConfigurationSuite) TestGetLogLevel() {
	key := configuration.EnvPrefix + "_" + "LOG_LEVEL"
	resetFunc := UnsetEnvVarAndRestore(s.T(), key)
	defer resetFunc()

	s.Run("default", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), configuration.DefaultLogLevel, config.GetLogLevel())
	})

	s.Run("file", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		newVal := u.String()
		config := s.getFileConfiguration(`log.level: "` + newVal + `"`)
		assert.Equal(s.T(), newVal, config.GetLogLevel())
	})

	s.Run("env overwrite", func() {
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		newVal := u.String()
		err = os.Setenv(key, newVal)
		require.NoError(s.T(), err)
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), newVal, config.GetLogLevel())
	})
}

func (s *TestConfigurationSuite) TestIsLogJSON() {
	key := configuration.EnvPrefix + "_" + "LOG_JSON"
	resetFunc := UnsetEnvVarAndRestore(s.T(), key)
	defer resetFunc()

	s.Run("default", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), configuration.DefaultLogJSON, config.IsLogJSON())
	})

	s.Run("file", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		newVal := !configuration.DefaultLogJSON
		config := s.getFileConfiguration(`log.json: "` + strconv.FormatBool(newVal) + `"`)
		assert.Equal(s.T(), newVal, config.IsLogJSON())
	})

	s.Run("env overwrite", func() {
		newVal := !configuration.DefaultLogJSON
		err := os.Setenv(key, strconv.FormatBool(newVal))
		require.NoError(s.T(), err)
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), newVal, config.IsLogJSON())
	})
}

func (s *TestConfigurationSuite) TestGetGracefulTimeout() {
	key := configuration.EnvPrefix + "_" + "GRACEFUL_TIMEOUT"

	s.Run("default", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), configuration.DefaultGracefulTimeout, config.GetGracefulTimeout())
	})

	s.Run("file", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		newVal := 333 * time.Second
		config := s.getFileConfiguration(`graceful_timeout: "` + newVal.String() + `"`)
		assert.Equal(s.T(), newVal, config.GetGracefulTimeout())
	})

	s.Run("env overwrite", func() {
		newVal := 666 * time.Second
		err := os.Setenv(key, newVal.String())
		require.NoError(s.T(), err)
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), newVal, config.GetGracefulTimeout())
	})
}

func (s *TestConfigurationSuite) TestGetHTTPWriteTimeout() {
	key := configuration.EnvPrefix + "_" + "HTTP_WRITE_TIMEOUT"

	s.Run("default", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), configuration.DefaultHTTPWriteTimeout, config.GetHTTPWriteTimeout())
	})

	s.Run("file", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		newVal := 333 * time.Second
		config := s.getFileConfiguration(`http.write_timeout: "` + newVal.String() + `"`)
		assert.Equal(s.T(), newVal, config.GetHTTPWriteTimeout())
	})

	s.Run("env overwrite", func() {
		newVal := 666 * time.Second
		err := os.Setenv(key, newVal.String())
		require.NoError(s.T(), err)
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), newVal, config.GetHTTPWriteTimeout())
	})
}

func (s *TestConfigurationSuite) TestGetHTTPReadTimeout() {
	key := configuration.EnvPrefix + "_" + "HTTP_READ_TIMEOUT"

	s.Run("default", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), configuration.DefaultHTTPReadTimeout, config.GetHTTPReadTimeout())
	})

	s.Run("file", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		newVal := 444 * time.Second
		config := s.getFileConfiguration(`http.read_timeout: "` + newVal.String() + `"`)
		assert.Equal(s.T(), newVal, config.GetHTTPReadTimeout())
	})

	s.Run("env overwrite", func() {
		newVal := 777 * time.Second
		err := os.Setenv(key, newVal.String())
		require.NoError(s.T(), err)
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), newVal, config.GetHTTPReadTimeout())
	})
}

func (s *TestConfigurationSuite) TestGetHTTPIdleTimeout() {
	key := configuration.EnvPrefix + "_" + "HTTP_IDLE_TIMEOUT"

	s.Run("default", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), configuration.DefaultHTTPIdleTimeout, config.GetHTTPIdleTimeout())
	})

	s.Run("file", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		newVal := 111 * time.Second
		config := s.getFileConfiguration(`http.idle_timeout: "` + newVal.String() + `"`)
		assert.Equal(s.T(), newVal, config.GetHTTPIdleTimeout())
	})

	s.Run("env overwrite", func() {
		newVal := 888 * time.Second
		err := os.Setenv(key, newVal.String())
		require.NoError(s.T(), err)
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), newVal, config.GetHTTPIdleTimeout())
	})
}

func (s *TestConfigurationSuite) TestGetHTTPCompressResponses() {
	key := configuration.EnvPrefix + "_" + "HTTP_COMPRESS"

	s.Run("default", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), configuration.DefaultHTTPCompressResponses, config.GetHTTPCompressResponses())
	})

	s.Run("file", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		newVal := !configuration.DefaultHTTPCompressResponses
		config := s.getFileConfiguration(`http.compress: "` + strconv.FormatBool(newVal) + `"`)
		assert.Equal(s.T(), newVal, config.GetHTTPCompressResponses())
	})

	s.Run("env overwrite", func() {
		newVal := !configuration.DefaultHTTPCompressResponses
		err := os.Setenv(key, strconv.FormatBool(newVal))
		require.NoError(s.T(), err)
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), newVal, config.GetHTTPCompressResponses())
	})
}

func (s *TestConfigurationSuite) TestGetEnvironmentAndTestingMode() {
	key := fmt.Sprintf("%s_ENVIRONMENT", configuration.EnvPrefix)
	resetFunc := UnsetEnvVarAndRestore(s.T(), key)
	defer resetFunc()

	s.Run("default", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), "prod", config.GetEnvironment())
		assert.False(s.T(), config.IsTestingMode())
	})

	s.Run("file", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getFileConfiguration("environment: TestGetEnvironmentFromConfig")
		assert.Equal(s.T(), "TestGetEnvironmentFromConfig", config.GetEnvironment())
		assert.False(s.T(), config.IsTestingMode())
	})

	s.Run("env overwrite", func() {
		err := os.Setenv(key, "TestGetEnvironmentFromEnvVar")
		require.NoError(s.T(), err)
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), "TestGetEnvironmentFromEnvVar", config.GetEnvironment())
		assert.False(s.T(), config.IsTestingMode())
	})

	s.Run("unit-tests env", func() {
		err := os.Setenv(key, "unit-tests")
		require.NoError(s.T(), err)
		config := s.getDefaultConfiguration()
		assert.True(s.T(), config.IsTestingMode())
	})
}

func (s *TestConfigurationSuite) TestGetAuthClientConfigRawPublicClient() {
	key := configuration.EnvPrefix + "_" + "AUTH_CLIENT_CONFIG_RAW_PUBLIC_CLIENT"

	s.Run("default", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), configuration.DefaultAuthClientConfigRawPublicClient, config.GetAuthClientConfigRawPublicClient())
	})

	s.Run("file", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		newVal := "false"
		config := s.getFileConfiguration(`auth_client.config_raw.public_client: "` + newVal + `"`)
		assert.Equal(s.T(), false, config.GetAuthClientConfigRawPublicClient())
	})

	s.Run("env overwrite", func() {
		newVal := "false"
		err := os.Setenv(key, newVal)
		require.NoError(s.T(), err)
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), false, config.GetAuthClientConfigRawPublicClient())
	})
}

func (s *TestConfigurationSuite) TestGetAuthClientConfigRawAuthServerURL() {
	key := configuration.EnvPrefix + "_" + "AUTH_CLIENT_CONFIG_RAW_AUTH_SERVER_URL"

	s.Run("default", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), configuration.DefaultAuthClientConfigRawAuthServerURL, config.GetAuthClientConfigRawAuthServerURL())
	})

	s.Run("file", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		newVal := u.String()
		config := s.getFileConfiguration(`auth_client.config_raw.auth_server_url: "` + newVal + `"`)
		assert.Equal(s.T(), newVal, config.GetAuthClientConfigRawAuthServerURL())
	})

	s.Run("env overwrite", func() {
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		newVal := u.String()
		err = os.Setenv(key, newVal)
		require.NoError(s.T(), err)
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), newVal, config.GetAuthClientConfigRawAuthServerURL())
	})
}

func (s *TestConfigurationSuite) TestGetAuthClientConfigRawSSLRequired() {
	key := configuration.EnvPrefix + "_" + "AUTH_CLIENT_CONFIG_RAW_SSL_REQUIRED"

	s.Run("default", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), configuration.DefaultAuthClientRawSSLRequired, config.GetAuthClientConfigRawSSLRequired())
	})

	s.Run("file", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		newVal := u.String()
		config := s.getFileConfiguration(`auth_client.config_raw.ssl_required: "` + newVal + `"`)
		assert.Equal(s.T(), newVal, config.GetAuthClientConfigRawSSLRequired())
	})

	s.Run("env overwrite", func() {
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		newVal := u.String()
		err = os.Setenv(key, newVal)
		require.NoError(s.T(), err)
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), newVal, config.GetAuthClientConfigRawSSLRequired())
	})
}

func (s *TestConfigurationSuite) TestGetAuthClientConfigRawRealm() {
	key := configuration.EnvPrefix + "_" + "AUTH_CLIENT_CONFIG_RAW_REALM"

	s.Run("default", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), configuration.DefaultAuthClientConfigRawRealm, config.GetAuthClientConfigRawRealm())
	})

	s.Run("file", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		newVal := u.String()
		config := s.getFileConfiguration(`auth_client.config_raw.realm: "` + newVal + `"`)
		assert.Equal(s.T(), newVal, config.GetAuthClientConfigRawRealm())
	})

	s.Run("env overwrite", func() {
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		newVal := u.String()
		err = os.Setenv(key, newVal)
		require.NoError(s.T(), err)
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), newVal, config.GetAuthClientConfigRawRealm())
	})
}

func (s *TestConfigurationSuite) TestGetAuthClientConfigRawResource() {
	key := configuration.EnvPrefix + "_" + "AUTH_CLIENT_CONFIG_RAW_RESOURCE"

	s.Run("default", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), configuration.DefaultAuthClientConfigRawResource, config.GetAuthClientConfigRawResource())
	})

	s.Run("file", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		newVal := u.String()
		config := s.getFileConfiguration(`auth_client.config_raw.resource: "` + newVal + `"`)
		assert.Equal(s.T(), newVal, config.GetAuthClientConfigRawResource())
	})

	s.Run("env overwrite", func() {
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		newVal := u.String()
		err = os.Setenv(key, newVal)
		require.NoError(s.T(), err)
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), newVal, config.GetAuthClientConfigRawResource())
	})
}

func (s *TestConfigurationSuite) TestGetAuthClientConfigRawClientID() {
	key := configuration.EnvPrefix + "_" + "AUTH_CLIENT_CONFIG_RAW_CLIENT_ID"

	s.Run("default", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), configuration.DefaultAuthClientConfigRawClientID, config.GetAuthClientConfigRawClientID())
	})

	s.Run("file", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		newVal := u.String()
		config := s.getFileConfiguration(`auth_client.config_raw.client_id: "` + newVal + `"`)
		assert.Equal(s.T(), newVal, config.GetAuthClientConfigRawClientID())
	})

	s.Run("env overwrite", func() {
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		newVal := u.String()
		err = os.Setenv(key, newVal)
		require.NoError(s.T(), err)
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), newVal, config.GetAuthClientConfigRawClientID())
	})
}

func (s *TestConfigurationSuite) TestGetAuthClientConfigVerificationEnabled() {
	key := configuration.EnvPrefix + "_" + "VERIFICATION_ENABLED"

	s.Run("default", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), configuration.DefaultVerificationEnabled, config.GetVerificationEnabled())
	})

	s.Run("file", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		newVal := "false"
		config := s.getFileConfiguration(`verification.enabled: "` + newVal + `"`)
		assert.Equal(s.T(), false, config.GetVerificationEnabled())
	})

	s.Run("env overwrite", func() {
		newVal := "false"
		err := os.Setenv(key, newVal)
		require.NoError(s.T(), err)
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), false, config.GetVerificationEnabled())
	})
}

func (s *TestConfigurationSuite) TestGetAuthClientConfigVerificationDailyLimit() {
	key := configuration.EnvPrefix + "_" + "VERIFICATION_DAILY_LIMIT"

	s.Run("default", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), configuration.DefaultVerificationDailyLimit, config.GetVerificationDailyLimit())
	})

	s.Run("file", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		newVal := "10"
		config := s.getFileConfiguration(`verification.daily_limit: "` + newVal + `"`)
		assert.Equal(s.T(), 10, config.GetVerificationDailyLimit())
	})

	s.Run("env overwrite", func() {
		newVal := "12"
		err := os.Setenv(key, newVal)
		require.NoError(s.T(), err)
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), 12, config.GetVerificationDailyLimit())
	})
}

func (s *TestConfigurationSuite) TestGetAuthClientConfigVerificationAttemptsAllowed() {
	key := configuration.EnvPrefix + "_" + "VERIFICATION_ATTEMPTS_ALLOWED"

	s.Run("default", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), configuration.DefaultVerificationAttemptsAllowed, config.GetVerificationAttemptsAllowed())
	})

	s.Run("file", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		newVal := "10"
		config := s.getFileConfiguration(`verification.attempts_allowed: "` + newVal + `"`)
		assert.Equal(s.T(), 10, config.GetVerificationAttemptsAllowed())
	})

	s.Run("env overwrite", func() {
		newVal := "12"
		err := os.Setenv(key, newVal)
		require.NoError(s.T(), err)
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), 12, config.GetVerificationAttemptsAllowed())
	})
}

func (s *TestConfigurationSuite) TestGetAuthClientConfigContentType() {
	key := configuration.EnvPrefix + "_" + "AUTH_CLIENT_CONFIG_CONTENT_TYPE"

	s.Run("default", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), configuration.DefaultAuthClientConfigContentType, config.GetAuthClientConfigAuthContentType())
	})

	s.Run("file", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		newVal := u.String()
		config := s.getFileConfiguration(`auth_client.config.content_type: "` + newVal + `"`)
		assert.Equal(s.T(), newVal, config.GetAuthClientConfigAuthContentType())
	})

	s.Run("env overwrite", func() {
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		newVal := u.String()
		err = os.Setenv(key, newVal)
		require.NoError(s.T(), err)
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), newVal, config.GetAuthClientConfigAuthContentType())
	})
}

func (s *TestConfigurationSuite) TestGetAuthClientLibraryURL() {
	key := configuration.EnvPrefix + "_" + "AUTH_CLIENT_LIBRARY_URL"
	resetFunc := UnsetEnvVarAndRestore(s.T(), key)
	defer resetFunc()

	s.Run("default", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), configuration.DefaultAuthClientLibraryURL, config.GetAuthClientLibraryURL())
	})

	s.Run("file", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		newVal := u.String()
		config := s.getFileConfiguration(`auth_client.library_url: "` + newVal + `"`)
		assert.Equal(s.T(), newVal, config.GetAuthClientLibraryURL())
	})

	s.Run("env overwrite", func() {
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		newVal := u.String()
		err = os.Setenv(key, newVal)
		require.NoError(s.T(), err)
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), newVal, config.GetAuthClientLibraryURL())
	})
}

func (s *TestConfigurationSuite) TestGetAuthClientPublicKeysURL() {
	key := configuration.EnvPrefix + "_" + "AUTH_CLIENT_PUBLIC_KEYS_URL"
	resetFunc := UnsetEnvVarAndRestore(s.T(), key)
	defer resetFunc()

	s.Run("default", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), configuration.DefaultAuthClientPublicKeysURL, config.GetAuthClientPublicKeysURL())
	})

	s.Run("file", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		newVal := u.String()
		config := s.getFileConfiguration(`auth_client.public_keys_url: "` + newVal + `"`)
		assert.Equal(s.T(), newVal, config.GetAuthClientPublicKeysURL())
	})

	s.Run("env overwrite", func() {
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		newVal := u.String()
		err = os.Setenv(key, newVal)
		require.NoError(s.T(), err)
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), newVal, config.GetAuthClientPublicKeysURL())
	})
}

func (s *TestConfigurationSuite) TestGetNamespace() {
	key := configuration.EnvPrefix + "_" + "NAMESPACE"
	resetFunc := UnsetEnvVarAndRestore(s.T(), key)
	defer resetFunc()

	s.Run("default", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), configuration.DefaultNamespace, config.GetNamespace())
	})

	s.Run("file", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		newVal := u.String()
		config := s.getFileConfiguration(`namespace: "` + newVal + `"`)
		assert.Equal(s.T(), newVal, config.GetNamespace())
	})

	s.Run("env overwrite", func() {
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		newVal := u.String()
		err = os.Setenv(key, newVal)
		require.NoError(s.T(), err)
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), newVal, config.GetNamespace())
	})
}

func (s *TestConfigurationSuite) TestVerificationEnabled() {
	key := configuration.EnvPrefix + "_" + "VERIFICATION_ENABLED"
	resetFunc := UnsetEnvVarAndRestore(s.T(), key)
	defer resetFunc()

	s.Run("default", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getDefaultConfiguration()
		require.True(s.T(), config.GetVerificationEnabled())
	})

	s.Run("file", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getFileConfiguration(`verification.enabled: "false"`)
		assert.False(s.T(), config.GetVerificationEnabled())
	})

	s.Run("env overwrite", func() {
		err := os.Setenv(key, "false")
		require.NoError(s.T(), err)
		config := s.getDefaultConfiguration()
		assert.False(s.T(), config.GetVerificationEnabled())
	})
}

func (s *TestConfigurationSuite) TestVerificationDailyLimit() {
	key := configuration.EnvPrefix + "_" + "VERIFICATION_DAILY_LIMIT"
	resetFunc := UnsetEnvVarAndRestore(s.T(), key)
	defer resetFunc()

	s.Run("default", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getDefaultConfiguration()
		require.Equal(s.T(), configuration.DefaultVerificationDailyLimit, config.GetVerificationDailyLimit())
	})

	s.Run("file", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getFileConfiguration(`verification.daily_limit: 2`)
		assert.Equal(s.T(), 2, config.GetVerificationDailyLimit())
	})

	s.Run("env overwrite", func() {
		newVal := "6"
		err := os.Setenv(key, newVal)
		require.NoError(s.T(), err)
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), 6, config.GetVerificationDailyLimit())
	})
}

func (s *TestConfigurationSuite) TestVerificationAttemptsAllowed() {
	key := configuration.EnvPrefix + "_" + "VERIFICATION_ATTEMPTS_ALLOWED"
	resetFunc := UnsetEnvVarAndRestore(s.T(), key)
	defer resetFunc()

	s.Run("default", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getDefaultConfiguration()
		require.Equal(s.T(), configuration.DefaultVerificationAttemptsAllowed, config.GetVerificationAttemptsAllowed())
	})

	s.Run("file", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getFileConfiguration(`verification.attempts_allowed: 4`)
		assert.Equal(s.T(), 4, config.GetVerificationAttemptsAllowed())
	})

	s.Run("env overwrite", func() {
		newVal := "2"
		err := os.Setenv(key, newVal)
		require.NoError(s.T(), err)
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), 2, config.GetVerificationAttemptsAllowed())
	})
}

func (s *TestConfigurationSuite) TestLoadSecret() {
	restore := SetEnvVarAndRestore(s.T(), "WATCH_NAMESPACE", "toolchain-host-operator")
	defer restore()
	s.T().Run("default", func(t *testing.T) {
		// when
		config := s.getDefaultConfiguration()

		// then
		assert.Equal(t, "", config.GetTwilioAccountSID())
		assert.Equal(t, "", config.GetTwilioAuthToken())
	})
	s.T().Run("env overwrite", func(t *testing.T) {
		// given
		restore := SetEnvVarAndRestore(t, "REGISTRATION_SERVICE_SECRET_NAME", "test-secret")
		defer restore()

		secret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret",
				Namespace: "toolchain-host-operator",
			},
			Data: map[string][]byte{
				"twilio.account.sid": []byte("test-account-sid"),
				"twilio.auth.token":  []byte("test-auth-token"),
			},
		}

		// when
		config, err := configuration.New("", NewFakeClient(t, secret))

		// then
		require.NoError(t, err)
		assert.Equal(t, "test-account-sid", config.GetTwilioAccountSID())
		assert.Equal(t, "test-auth-token", config.GetTwilioAuthToken())
	})

	s.T().Run("secret not found", func(t *testing.T) {
		// given
		restore := SetEnvVarAndRestore(t, "REGISTRATION_SERVICE_SECRET_NAME", "test-secret")
		defer restore()

		// when
		config, err := configuration.New("", NewFakeClient(t))

		// then
		require.NoError(t, err)
		assert.NotNil(t, config)
	})
}

func (s *TestConfigurationSuite) TestLoadConfigMap() {
	restore := SetEnvVarAndRestore(s.T(), "WATCH_NAMESPACE", "toolchain-host-operator")
	defer restore()
	s.T().Run("default", func(t *testing.T) {
		// when
		config := s.getDefaultConfiguration()

		// then
		assert.Equal(t, "", config.GetTwilioAccountSID())
		assert.Equal(t, "", config.GetTwilioAuthToken())
	})
	s.T().Run("env overwrite", func(t *testing.T) {
		// given
		restore := SetEnvVarAndRestore(t, "REGISTRATION_SERVICE_CONFIG_MAP_NAME", "test-config-map")
		defer restore()

		configMap := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-config-map",
				Namespace: "toolchain-host-operator",
			},
			Data: map[string]string{
				"auth_client.config_raw.realm":           "test-realm",
				"auth_client.config_raw.auth_server_url": "test-auth-server-url",
				"auth_client.config_raw.ssl_required":    "test-ssl-required",
				"auth_client.config_raw.resource":        "test-resource",
				"auth_client.config_raw.client_id":       "test-client-id",
				"auth_client.config_raw.public_client":   "true",
				"verification.enabled":                   "false",
				"verification.daily_limit":               "20",
				"verification.attempts_allowed":          "100",
			},
		}

		// when
		config, err := configuration.New("", NewFakeClient(t, configMap))

		// then
		require.NoError(t, err)
		assert.Equal(t, "test-realm", config.GetAuthClientConfigRawRealm())
		assert.Equal(t, "test-auth-server-url", config.GetAuthClientConfigRawAuthServerURL())
		assert.Equal(t, "test-ssl-required", config.GetAuthClientConfigRawSSLRequired())
		assert.Equal(t, "test-resource", config.GetAuthClientConfigRawResource())
		assert.Equal(t, "test-client-id", config.GetAuthClientConfigRawClientID())
		assert.Equal(t, true, config.GetAuthClientConfigRawPublicClient())
		assert.Equal(t, false, config.GetVerificationEnabled())
		assert.Equal(t, 20, config.GetVerificationDailyLimit())
		assert.Equal(t, 100, config.GetVerificationAttemptsAllowed())
	})

	s.T().Run("secret not found", func(t *testing.T) {
		// given
		restore := SetEnvVarAndRestore(t, "REGISTRATION_SERVICE_CONFIG_MAP_NAME", "test-config-map")
		defer restore()

		// when
		config, err := configuration.New("", NewFakeClient(t))

		// then
		require.NoError(t, err)
		assert.NotNil(t, config)
	})
}
