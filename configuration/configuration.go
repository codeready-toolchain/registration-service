// Package configuration is in charge of the validation and extraction of all
// the configuration details from a configuration file or environment variables.
package configuration

import (
	"strings"
	"time"

	errs "github.com/pkg/errors"
	"github.com/spf13/viper"
)

const (
	// EnvPrefix will be used for environment variable name prefixing.
	EnvPrefix = "APP"

	// Constants for viper variable names. Will be used to set
	// default values as well as to get each value
	varHTTPAddress = "http.address"
	// DefaultHTTPAddress is the address and port string that your service will
	// be exported to by default.
	DefaultHTTPAddress = "0.0.0.0:8080"

	varHTTPIdleTimeout = "http.idle_timeout"
	// DefaultHTTPIdleTimeout specifies the default timeout for HTTP idling
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
	// wait for existing connections to finish - e.g. 15s or 1m
	DefaultGracefulTimeout = time.Second * 15

	varHTTPWriteTimeout = "http.write_timeout"
	// DefaultHTTPWriteTimeout specifies the default timeout for HTTP writes
	DefaultHTTPWriteTimeout = time.Second * 15

	varHTTPReadTimeout = "http.read_timeout"
	// DefaultHTTPReadTimeout specifies the default timeout for HTTP reads
	DefaultHTTPReadTimeout = time.Second * 15
)

// Registry encapsulates the Viper configuration registry which stores the
// configuration data in-memory.
type Registry struct {
	v *viper.Viper
}

// New creates a configuration reader object using a configurable configuration
// file path. If the provided config file path is empty, a default configuration
// will be created.
func New(configFilePath string) (*Registry, error) {
	c := Registry{
		v: viper.New(),
	}
	c.v.SetEnvPrefix(EnvPrefix)
	c.v.AutomaticEnv()
	c.v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	c.v.SetTypeByDefaultValue(true)
	c.setConfigDefaults()

	if configFilePath != "" {
		c.v.SetConfigType("yaml")
		c.v.SetConfigFile(configFilePath)
		err := c.v.ReadInConfig() // Find and read the config file
		if err != nil {           // Handle errors reading the config file
			return nil, errs.Wrap(err, "failed to read config file")
		}
	}
	return &c, nil
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
}

// GetHTTPAddress returns the HTTP address (as set via default, config file, or
// environment variable) that the app-server binds to (e.g. "0.0.0.0:8080")
func (c *Registry) GetHTTPAddress() string {
	return c.v.GetString(varHTTPAddress)
}

// GetHTTPCompressResponses returns true if HTTP responses should be compressed
// for clients that support it via the 'Accept-Encoding' header.
func (c *Registry) GetHTTPCompressResponses() bool {
	return c.v.GetBool(varHTTPCompressResponses)
}

// GetHTTPWriteTimeout returns the duration for which
func (c *Registry) GetHTTPWriteTimeout() time.Duration {
	return c.v.GetDuration(varHTTPWriteTimeout)
}

// GetHTTPReadTimeout returns the duration for which
func (c *Registry) GetHTTPReadTimeout() time.Duration {
	return c.v.GetDuration(varHTTPReadTimeout)
}

// GetHTTPIdleTimeout returns the duration for which
func (c *Registry) GetHTTPIdleTimeout() time.Duration {
	return c.v.GetDuration(varHTTPIdleTimeout)
}

// GetLogLevel returns the loggging level (as set via config file or environment
// variable)
func (c *Registry) GetLogLevel() string {
	return c.v.GetString(varLogLevel)
}

// IsLogJSON returns if we should log json format (as set via config file or
// environment variable)
func (c *Registry) IsLogJSON() bool {
	return c.v.GetBool(varLogJSON)
}

// GetGracefulTimeout returns the duration for which the server gracefully wait
// for existing connections to finish - e.g. 15s or 1m
func (c *Registry) GetGracefulTimeout() time.Duration {
	return c.v.GetDuration(varGracefulTimeout)
}
