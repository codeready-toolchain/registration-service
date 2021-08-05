// Package configuration is in charge of the validation and extraction of all
// the configuration details from a configuration file or environment variables.
package configuration

import (
	"os"
	"time"

	commonconfig "github.com/codeready-toolchain/toolchain-common/pkg/configuration"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("configuration")

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
	GracefulTimeout       = time.Second * 15
	HTTPAddress           = "0.0.0.0:8080"
	HTTPCompressResponses = true
	HTTPIdleTimeout       = time.Second * 15
	HTTPReadTimeout       = time.Second * 15
	HTTPWriteTimeout      = time.Second * 15

	DefaultEnvironment   = "prod"
	UnitTestsEnvironment = "unit-tests"
)

func IsTestingMode() bool {
	cfg := commonconfig.GetCachedToolchainConfig()
	return cfg.RegistrationService().Environment() == UnitTestsEnvironment
}

func Namespace() string {
	return os.Getenv("WATCH_NAMESPACE")
}
