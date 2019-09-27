// Package configuration is in charge of the validation and extraction of all
// the configuration details from a configuration file or environment variables.
package configuration

import (
	"strings"
	"time"

	errs "github.com/pkg/errors"
	"github.com/spf13/viper"
)

var (
	// Commit current build commit set by build script.
	Commit = "0"
	// BuildTime set by build script in ISO 8601 (UTC) format:
	// YYYY-MM-DDThh:mm:ssTZD (see https://www.w3.org/TR/NOTE-datetime for
	// details).
	BuildTime = "0"
	// StartTime in ISO 8601 (UTC) format.
	StartTime = time.Now().UTC().Format("2006-01-02T15:04:05Z")
)

const (
	// EnvPrefix will be used for environment variable name prefixing.
	EnvPrefix = "REGISTRATION"

	// Constants for viper variable names. Will be used to set
	// default values as well as to get each value.
	varHTTPAddress = "http.address"
	// DefaultHTTPAddress is the address and port string that your service will
	// be exported to by default.
	DefaultHTTPAddress = "0.0.0.0:8080"

	varHTTPIdleTimeout = "http.idle_timeout"
	// DefaultHTTPIdleTimeout specifies the default timeout for HTTP idling.
	DefaultHTTPIdleTimeout = time.Second * 15

	varHTTPCompressResponses = "http.compress"
	// DefaultHTTPCompressResponses compresses HTTP responses for clients that
	// support it via the 'Accept-Encoding' header.
	DefaultHTTPCompressResponses = false

	varLogLevel = "log.level"
	// DefaultLogLevel is the default log level used in your service.
	DefaultLogLevel = "info"

	varLogJSON = "log.json"
	// DefaultLogJSON is a switch to toggle on and off JSON log output.
	DefaultLogJSON = false

	varGracefulTimeout = "graceful_timeout"
	// DefaultGracefulTimeout is the duration for which the server gracefully
	// wait for existing connections to finish - e.g. 15s or 1m.
	DefaultGracefulTimeout = time.Second * 15

	varHTTPWriteTimeout = "http.write_timeout"
	// DefaultHTTPWriteTimeout specifies the default timeout for HTTP writes.
	DefaultHTTPWriteTimeout = time.Second * 15

	varHTTPReadTimeout = "http.read_timeout"
	// DefaultHTTPReadTimeout specifies the default timeout for HTTP reads.
	DefaultHTTPReadTimeout = time.Second * 15

	varTestingMode = "testingmode"
	// DefaultTestingMode specifies whether the services should run in testing mode.
	DefaultTestingMode = false

	varAuthClientLibraryURL = "auth_client.library_url"
	// DefaultAuthClientLibraryURL is the default auth library location.
	DefaultAuthClientLibraryURL = "https://keycloak.service/auth/js/keycloak.js"

	varAuthClientConfigRaw = "auth_client.config.raw"
	// DefaultAuthClientConfigRaw specifies the auth client config.
	DefaultAuthClientConfigRaw = `{"realm":"myRealm","auth-server-url":"https://auth.service/auth","ssl-required": "none", "resource": "registrationService","public-client": true,"confidential-port": 0}`

	varAuthClientConfigContentType = "auth_client.config.content_type"
	// DefaultAuthClientConfigContentType specifies the auth client config content type.
	DefaultAuthClientConfigContentType = "application/json"

	varAuthClientPublicKeysURL = "auth_client.public_keys_url"

	// DefaultAuthClientPublicKeysURL is the default log level used in your service.
	DefaultAuthClientPublicKeysURL = ""

	varNamespace = "namespace"
)

// Registry encapsulates the Viper configuration registry which stores the
// configuration data in-memory.
type Registry struct {
	v *viper.Viper
}

// CreateEmptyRegistry creates an initial, empty registry.
func CreateEmptyRegistry() *Registry {
	c := Registry{
		v: viper.New(),
	}
	c.v.SetEnvPrefix(EnvPrefix)
	c.v.AutomaticEnv()
	c.v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	c.v.SetTypeByDefaultValue(true)
	c.setConfigDefaults()
	return &c
}

// New creates a configuration reader object using a configurable configuration
// file path. If the provided config file path is empty, a default configuration
// will be created.
func New(configFilePath string) (*Registry, error) {
	c := CreateEmptyRegistry()
	if configFilePath != "" {
		c.v.SetConfigType("yaml")
		c.v.SetConfigFile(configFilePath)
		err := c.v.ReadInConfig() // Find and read the config file
		if err != nil {           // Handle errors reading the config file.
			return nil, errs.Wrap(err, "failed to read config file")
		}
	}
	return c, nil
}

// GetViperInstance returns the underlying Viper instance.
func (c *Registry) GetViperInstance() *viper.Viper {
	return c.v
}

func (c *Registry) setConfigDefaults() {
	c.v.SetTypeByDefaultValue(true)

	c.v.SetDefault(varHTTPAddress, DefaultHTTPAddress)
	c.v.SetDefault(varHTTPCompressResponses, DefaultHTTPCompressResponses)
	c.v.SetDefault(varHTTPWriteTimeout, DefaultHTTPWriteTimeout)
	c.v.SetDefault(varHTTPReadTimeout, DefaultHTTPReadTimeout)
	c.v.SetDefault(varHTTPIdleTimeout, DefaultHTTPIdleTimeout)
	c.v.SetDefault(varLogLevel, DefaultLogLevel)
	c.v.SetDefault(varLogJSON, DefaultLogJSON)
	c.v.SetDefault(varGracefulTimeout, DefaultGracefulTimeout)
	c.v.SetDefault(varTestingMode, DefaultTestingMode)
	c.v.SetDefault(varAuthClientLibraryURL, DefaultAuthClientLibraryURL)
	c.v.SetDefault(varAuthClientConfigRaw, DefaultAuthClientConfigRaw)
	c.v.SetDefault(varAuthClientConfigContentType, DefaultAuthClientConfigContentType)
	c.v.SetDefault(varAuthClientPublicKeysURL, DefaultAuthClientPublicKeysURL)
}

// GetHTTPAddress returns the HTTP address (as set via default, config file, or
// environment variable) that the app-server binds to (e.g. "0.0.0.0:8080").
func (c *Registry) GetHTTPAddress() string {
	return c.v.GetString(varHTTPAddress)
}

// GetHTTPCompressResponses returns true if HTTP responses should be compressed
// for clients that support it via the 'Accept-Encoding' header.
func (c *Registry) GetHTTPCompressResponses() bool {
	return c.v.GetBool(varHTTPCompressResponses)
}

// GetHTTPWriteTimeout returns the duration for the write timeout.
func (c *Registry) GetHTTPWriteTimeout() time.Duration {
	return c.v.GetDuration(varHTTPWriteTimeout)
}

// GetHTTPReadTimeout returns the duration for the read timeout.
func (c *Registry) GetHTTPReadTimeout() time.Duration {
	return c.v.GetDuration(varHTTPReadTimeout)
}

// GetHTTPIdleTimeout returns the duration for the idle timeout.
func (c *Registry) GetHTTPIdleTimeout() time.Duration {
	return c.v.GetDuration(varHTTPIdleTimeout)
}

// GetLogLevel returns the loggging level (as set via config file or environment
// variable).
func (c *Registry) GetLogLevel() string {
	return c.v.GetString(varLogLevel)
}

// IsLogJSON returns if we should log json format (as set via config file or
// environment variable).
func (c *Registry) IsLogJSON() bool {
	return c.v.GetBool(varLogJSON)
}

// GetGracefulTimeout returns the duration for which the server gracefully wait
// for existing connections to finish - e.g. 15s or 1m.
func (c *Registry) GetGracefulTimeout() time.Duration {
	return c.v.GetDuration(varGracefulTimeout)
}

// IsTestingMode returns if the service should run in testing mode (as set via
// config file or environment variable).
func (c *Registry) IsTestingMode() bool {
	return c.v.GetBool(varTestingMode)
}

// GetAuthClientLibraryURL returns the auth library location (as set via
// config file or environment variable).
func (c *Registry) GetAuthClientLibraryURL() string {
	return c.v.GetString(varAuthClientLibraryURL)
}

// GetAuthClientConfigAuthContentType returns the auth config config content type (as
// set via config file or environment variable).
func (c *Registry) GetAuthClientConfigAuthContentType() string {
	return c.v.GetString(varAuthClientConfigContentType)
}

// GetAuthClientConfigAuthRaw returns the auth config config (as
// set via config file or environment variable).
func (c *Registry) GetAuthClientConfigAuthRaw() string {
	return c.v.GetString(varAuthClientConfigRaw)
}

// GetAuthClientPublicKeysURL returns the public keys URL (as set via config file
// or environment variable).
func (c *Registry) GetAuthClientPublicKeysURL() string {
	return c.v.GetString(varAuthClientPublicKeysURL)
}

// GetNamespace returns the namespace in which the registration service and host operator is running
func (c *Registry) GetNamespace() string {
	return c.v.GetString(varNamespace)
}
