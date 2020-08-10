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
	DefaultHTTPCompressResponses = true

	// varEnvironment specifies service environment such as prod, stage, unit-tests, e2e-tests, dev, etc
	varEnvironment = "environment"
	// DefaultEnvironment is the default environment
	DefaultEnvironment   = "prod"
	UnitTestsEnvironment = "unit-tests"

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

	varAuthClientLibraryURL = "auth_client.library_url"
	// DefaultAuthClientLibraryURL is the default auth library location.
	DefaultAuthClientLibraryURL = "https://sso.prod-preview.openshift.io/auth/js/keycloak.js"

	varAuthClientConfigRawRealm        = "auth_client.config_raw.realm"
	varAuthClientConfigRawServerURL    = "auth_client.config_raw.auth_server_url"
	varAuthClientConfigRawSSLRequired  = "auth_client.config_raw.ssl-required"
	varAuthClientConfigRawResource     = "auth_client.config_raw.resource"
	varAuthClientConfigRawClientID     = "auth_client.config_raw.clientId"
	varAuthClientConfigRawPublicClient = "auth_client.config_raw.public_client"

	varAuthClientConfigContentType = "auth_client.config.content_type"
	// DefaultAuthClientConfigContentType specifies the auth client config content type.
	DefaultAuthClientConfigContentType = "application/json; charset=utf-8"

	varAuthClientPublicKeysURL = "auth_client.public_keys_url"
	// DefaultAuthClientPublicKeysURL is the default log level used in your service.
	DefaultAuthClientPublicKeysURL = "https://sso.prod-preview.openshift.io/auth/realms/toolchain-public/protocol/openid-connect/certs"

	varNamespace = "namespace"
	// DefaultNamespace is the default k8s namespace to use.
	DefaultNamespace = "toolchain-host-operator"
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
	c.v.SetDefault(varEnvironment, DefaultEnvironment)
	c.v.SetDefault(varLogLevel, DefaultLogLevel)
	c.v.SetDefault(varLogJSON, DefaultLogJSON)
	c.v.SetDefault(varGracefulTimeout, DefaultGracefulTimeout)
	c.v.SetDefault(varAuthClientLibraryURL, DefaultAuthClientLibraryURL)
	c.v.SetDefault(varAuthClientConfigContentType, DefaultAuthClientConfigContentType)
	c.v.SetDefault(varAuthClientPublicKeysURL, DefaultAuthClientPublicKeysURL)
	c.v.SetDefault(varNamespace, DefaultNamespace)

	c.v.SetDefault(varAuthClientConfigRawRealm, "")
	c.v.SetDefault(varAuthClientConfigRawServerURL, "")
	c.v.SetDefault(varAuthClientConfigRawSSLRequired, "")
	c.v.SetDefault(varAuthClientConfigRawResource, "")
	c.v.SetDefault(varAuthClientConfigRawClientID, "")
	c.v.SetDefault(varAuthClientConfigRawPublicClient, "")
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

// GetEnvironment returns the environment such as prod, stage, unit-tests, e2e-tests, dev, etc
func (c *Registry) GetEnvironment() string {
	return c.v.GetString(varEnvironment)
}

// GetLogLevel returns the logging level (as set via config file or environment
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

// IsTestingMode returns if the service runs in unit-tests environment
func (c *Registry) IsTestingMode() bool {
	return c.GetEnvironment() == UnitTestsEnvironment
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
func (c *Registry) GetAuthClientConfigAuthRawClientID() string {
	return c.v.GetString(varAuthClientConfigRawClientID)
}

func (c *Registry) GetAuthClientConfigAuthRawPublicClient() string {
	return c.v.GetString(varAuthClientConfigRawPublicClient)
}

func (c *Registry) GetAuthClientConfigAuthRawRealm() string {
	return c.v.GetString(varAuthClientConfigRawRealm)
}

func (c *Registry) GetAuthClientConfigAuthRawResource() string {
	return c.v.GetString(varAuthClientConfigRawResource)
}

func (c *Registry) GetAuthClientConfigAuthRawServerURL() string {
	return c.v.GetString(varAuthClientConfigRawServerURL)
}

func (c *Registry) GetAuthClientConfigAuthRawSSLReuired() string {
	return c.v.GetString(varAuthClientConfigRawSSLRequired)
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
