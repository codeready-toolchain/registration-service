// Package configuration is in charge of the validation and extraction of all
// the configuration details from a configuration file or environment variables.
package configuration

import (
	"fmt"
	"os"
	"strings"
	"time"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	commonconfig "github.com/codeready-toolchain/toolchain-common/pkg/configuration"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
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

var logger = logf.Log.WithName("configuration")

const (
	GracefulTimeout       = time.Second * 15
	HTTPAddress           = "0.0.0.0:8080"
	HTTPCompressResponses = true
	HTTPIdleTimeout       = time.Second * 15
	HTTPReadTimeout       = time.Second * 15
	HTTPWriteTimeout      = time.Second * 15

	prodEnvironment      = "prod"
	DefaultEnvironment   = prodEnvironment
	UnitTestsEnvironment = "unit-tests"
)

var configurationClient client.Client

func IsTestingMode() bool {
	cfg := GetRegistrationServiceConfig()
	return cfg.Environment() == UnitTestsEnvironment
}

func Namespace() string {
	return os.Getenv(commonconfig.WatchNamespaceEnvVar)
}

// GetRegistrationServiceConfig returns a RegistrationServiceConfig using the cache, or if the cache was not initialized
// then retrieves the latest config using the provided client and updates the cache
func GetRegistrationServiceConfig() RegistrationServiceConfig {
	if configurationClient == nil {
		logger.Error(fmt.Errorf("configuration client is not initialized"), "using default configuration")
		return RegistrationServiceConfig{cfg: &toolchainv1alpha1.ToolchainConfigSpec{}}
	}
	config, secrets, err := commonconfig.GetConfig(configurationClient, &toolchainv1alpha1.ToolchainConfig{})
	if err != nil {
		// return default config
		logger.Error(err, "failed to retrieve RegistrationServiceConfig, using default configuration")
		return RegistrationServiceConfig{cfg: &toolchainv1alpha1.ToolchainConfigSpec{}}
	}
	return NewRegistrationServiceConfig(config, secrets)
}

// ForceLoadRegistrationServiceConfig updates the cache using the provided client and returns the latest RegistrationServiceConfig
func ForceLoadRegistrationServiceConfig(cl client.Client) (RegistrationServiceConfig, error) {
	if configurationClient == nil {
		configurationClient = cl
	}
	config, secrets, err := commonconfig.LoadLatest(cl, &toolchainv1alpha1.ToolchainConfig{})
	if err != nil {
		// return default config
		logger.Error(err, "failed to force load RegistrationServiceConfig")
		return RegistrationServiceConfig{cfg: &toolchainv1alpha1.ToolchainConfigSpec{}}, err
	}
	return NewRegistrationServiceConfig(config, secrets), nil
}

type RegistrationServiceConfig struct {
	cfg     *toolchainv1alpha1.ToolchainConfigSpec
	secrets map[string]map[string]string
}

func NewRegistrationServiceConfig(config runtime.Object, secrets map[string]map[string]string) RegistrationServiceConfig {
	if config == nil {
		// return default config if there's no config resource
		return RegistrationServiceConfig{cfg: &toolchainv1alpha1.ToolchainConfigSpec{}}
	}

	toolchaincfg, ok := config.(*toolchainv1alpha1.ToolchainConfig)
	if !ok {
		// return default config
		logger.Error(fmt.Errorf("cache does not contain toolchainconfig resource type"), "failed to get ToolchainConfig from resource, using default configuration")
		return RegistrationServiceConfig{cfg: &toolchainv1alpha1.ToolchainConfigSpec{}}
	}
	return RegistrationServiceConfig{cfg: &toolchaincfg.Spec, secrets: secrets}
}

func (r RegistrationServiceConfig) Print() {
	if r.cfg == nil {
		logger.Info("ToolchainConfig not found, using default Registration Service configuration")
		return
	}
	logger.Info("Registration Service Configuration", "config", r.cfg.Host.RegistrationService)
}

func (r RegistrationServiceConfig) Environment() string {
	return commonconfig.GetString(r.cfg.Host.RegistrationService.Environment, prodEnvironment)
}

func (r RegistrationServiceConfig) IsProdEnvironment() bool {
	return r.Environment() == prodEnvironment
}

func (r RegistrationServiceConfig) Analytics() AnalyticsConfig {
	return AnalyticsConfig{r.cfg.Host.RegistrationService.Analytics}
}

func (r RegistrationServiceConfig) Auth() AuthConfig {
	return AuthConfig{r.cfg.Host.RegistrationService.Auth}
}

func (r RegistrationServiceConfig) LogLevel() string {
	return commonconfig.GetString(r.cfg.Host.RegistrationService.LogLevel, "info")
}

func (r RegistrationServiceConfig) Namespace() string {
	return commonconfig.GetString(r.cfg.Host.RegistrationService.Namespace, "toolchain-host-operator")
}

func (r RegistrationServiceConfig) RegistrationServiceURL() string {
	return commonconfig.GetString(r.cfg.Host.RegistrationService.RegistrationServiceURL, "https://registration.crt-placeholder.com")
}

func (r RegistrationServiceConfig) Verification() VerificationConfig {
	return VerificationConfig{c: r.cfg.Host.RegistrationService.Verification, secrets: r.secrets}
}

type AnalyticsConfig struct {
	c toolchainv1alpha1.RegistrationServiceAnalyticsConfig
}

func (r AnalyticsConfig) WoopraDomain() string {
	return commonconfig.GetString(r.c.WoopraDomain, "")
}

func (r AnalyticsConfig) SegmentWriteKey() string {
	return commonconfig.GetString(r.c.SegmentWriteKey, "")
}

type AuthConfig struct {
	c toolchainv1alpha1.RegistrationServiceAuthConfig
}

func (r AuthConfig) AuthClientLibraryURL() string {
	return commonconfig.GetString(r.c.AuthClientLibraryURL, "https://sso.devsandbox.dev/auth/js/keycloak.js")
}

func (r AuthConfig) AuthClientConfigContentType() string {
	return commonconfig.GetString(r.c.AuthClientConfigContentType, "application/json; charset=utf-8")
}

func (r AuthConfig) AuthClientConfigRaw() string {
	return commonconfig.GetString(r.c.AuthClientConfigRaw, `{"realm": "sandbox-dev","auth-server-url": "https://sso.devsandbox.dev/auth","ssl-required": "none","resource": "sandbox-public","clientId": "sandbox-public","public-client": true, "confidential-port": 0}`)
}

func (r AuthConfig) AuthClientPublicKeysURL() string {
	return commonconfig.GetString(r.c.AuthClientPublicKeysURL, "https://sso.devsandbox.dev/auth/realms/sandbox-dev/protocol/openid-connect/certs")
}

type VerificationConfig struct {
	c       toolchainv1alpha1.RegistrationServiceVerificationConfig
	secrets map[string]map[string]string
}

func (r VerificationConfig) registrationServiceSecret(secretKey string) string {
	secret := commonconfig.GetString(r.c.Secret.Ref, "")
	return r.secrets[secret][secretKey]
}

func (r VerificationConfig) Enabled() bool {
	return commonconfig.GetBool(r.c.Enabled, false)
}

func (r VerificationConfig) DailyLimit() int {
	return commonconfig.GetInt(r.c.DailyLimit, 5)
}

func (r VerificationConfig) AttemptsAllowed() int {
	return commonconfig.GetInt(r.c.AttemptsAllowed, 3)
}

func (r VerificationConfig) MessageTemplate() string {
	return commonconfig.GetString(r.c.MessageTemplate, "Developer Sandbox for Red Hat OpenShift: Your verification code is %s")
}

func (r VerificationConfig) ExcludedEmailDomains() []string {
	excluded := commonconfig.GetString(r.c.ExcludedEmailDomains, "")
	v := strings.FieldsFunc(excluded, func(c rune) bool {
		return c == ','
	})
	return v
}

func (r VerificationConfig) CodeExpiresInMin() int {
	return commonconfig.GetInt(r.c.CodeExpiresInMin, 5)
}

func (r VerificationConfig) TwilioAccountSID() string {
	key := commonconfig.GetString(r.c.Secret.TwilioAccountSID, "")
	return r.registrationServiceSecret(key)
}

func (r VerificationConfig) TwilioAuthToken() string {
	key := commonconfig.GetString(r.c.Secret.TwilioAuthToken, "")
	return r.registrationServiceSecret(key)
}

func (r VerificationConfig) TwilioFromNumber() string {
	key := commonconfig.GetString(r.c.Secret.TwilioFromNumber, "")
	return r.registrationServiceSecret(key)
}
