package configuration_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"testing"
	"time"

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
	config, err := configuration.New("")
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
	config, err := configuration.New(tmpFile.Name())
	require.NoError(s.T(), err)
	return config
}

func (s *TestConfigurationSuite) TestNew() {
	s.Run("default configuration", func() {
		reg, err := configuration.New("")
		require.NoError(s.T(), err)
		require.NotNil(s.T(), reg)
	})
	s.Run("non existing file path", func() {
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		reg, err := configuration.New(u.String())
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

func (s *TestConfigurationSuite) TestGetAuthClientConfigRaw() {
	key := configuration.EnvPrefix + "_" + "AUTH_CLIENT_CONFIG_RAW"

	s.Run("default", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), configuration.DefaultAuthClientConfigRaw, config.GetAuthClientConfigAuthRaw())
	})

	s.Run("file", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		newVal := u.String()
		config := s.getFileConfiguration(`auth_client.config.raw: "` + newVal + `"`)
		assert.Equal(s.T(), newVal, config.GetAuthClientConfigAuthRaw())
	})

	s.Run("env overwrite", func() {
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		newVal := u.String()
		err = os.Setenv(key, newVal)
		require.NoError(s.T(), err)
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), newVal, config.GetAuthClientConfigAuthRaw())
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
		require.Equal(s.T(), true, config.GetVerificationEnabled())
	})

	s.Run("file", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getFileConfiguration(`verification.enabled: "` + strconv.FormatBool(false) + `"`)
		assert.Equal(s.T(), false, config.GetVerificationEnabled())
	})

	s.Run("env overwrite", func() {
		newVal := strconv.FormatBool(false)
		err := os.Setenv(key, newVal)
		require.NoError(s.T(), err)
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), false, config.GetVerificationEnabled())
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

func (s *TestConfigurationSuite) TestTwilioAccountSID() {
	key := configuration.EnvPrefix + "_" + "TWILIO_ACCOUNT_SID"
	resetFunc := UnsetEnvVarAndRestore(s.T(), key)
	defer resetFunc()

	s.Run("default", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getDefaultConfiguration()
		require.Equal(s.T(), "", config.GetTwilioAccountSID())
	})

	s.Run("file", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		config := s.getFileConfiguration(`twilio.account_sid: ` + u.String())
		assert.Equal(s.T(), u.String(), config.GetTwilioAccountSID())
	})

	s.Run("env overwrite", func() {
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		err = os.Setenv(key, u.String())
		require.NoError(s.T(), err)
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), u.String(), config.GetTwilioAccountSID())
	})
}

func (s *TestConfigurationSuite) TestTwilioAuthToken() {
	key := configuration.EnvPrefix + "_" + "TWILIO_AUTH_TOKEN"
	resetFunc := UnsetEnvVarAndRestore(s.T(), key)
	defer resetFunc()

	s.Run("default", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		config := s.getDefaultConfiguration()
		require.Equal(s.T(), "", config.GetTwilioAuthToken())
	})

	s.Run("file", func() {
		resetFunc := UnsetEnvVarAndRestore(s.T(), key)
		defer resetFunc()
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		config := s.getFileConfiguration(`twilio.auth_token: ` + u.String())
		assert.Equal(s.T(), u.String(), config.GetTwilioAuthToken())
	})

	s.Run("env overwrite", func() {
		u, err := uuid.NewV4()
		require.NoError(s.T(), err)
		err = os.Setenv(key, u.String())
		require.NoError(s.T(), err)
		config := s.getDefaultConfiguration()
		assert.Equal(s.T(), u.String(), config.GetTwilioAuthToken())
	})
}
