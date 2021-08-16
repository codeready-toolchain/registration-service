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

	DefaultEnvironment   = "prod"
	UnitTestsEnvironment = "unit-tests"
)

func IsTestingMode() bool {
	cfg := GetCachedRegistrationServiceConfig()
	return cfg.Environment() == UnitTestsEnvironment
}

func Namespace() string {
	return os.Getenv(commonconfig.WatchNamespaceEnvVar)
}

// GetRegistrationServiceConfig returns a RegistrationServiceConfig using the cache, or if the cache was not initialized
// then retrieves the latest config using the provided client and updates the cache
func GetRegistrationServiceConfig(cl client.Client) (RegistrationServiceConfig, error) {
	config, secrets, err := commonconfig.GetConfig(cl, &toolchainv1alpha1.ToolchainConfig{})
	if err != nil {
		// return default config
		logger.Error(err, "failed to retrieve RegistrationServiceConfig")
		return RegistrationServiceConfig{cfg: &toolchainv1alpha1.ToolchainConfigSpec{}}, err
	}
	return NewRegistrationServiceConfig(config, secrets), nil
}

// GetCachedRegistrationServiceConfig returns a RegistrationServiceConfig directly from the cache
func GetCachedRegistrationServiceConfig() RegistrationServiceConfig {
	config, secrets := commonconfig.GetCachedConfig()
	return NewRegistrationServiceConfig(config, secrets)
}

// ForceLoadRegistrationServiceConfig updates the cache using the provided client and returns the latest RegistrationServiceConfig
func ForceLoadRegistrationServiceConfig(cl client.Client) (RegistrationServiceConfig, error) {
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

func (r *RegistrationServiceConfig) Print() {
	logger.Info("Registration Service Configuration variables", "ToolchainConfigSpec", r.cfg.Host.RegistrationService)
}

func (r *RegistrationServiceConfig) Environment() string {
	return commonconfig.GetString(r.cfg.Host.RegistrationService.Environment, "prod")
}

func (r RegistrationServiceConfig) Analytics() RegistrationServiceAnalyticsConfig {
	return RegistrationServiceAnalyticsConfig{r.cfg.Host.RegistrationService.Analytics}
}

func (r RegistrationServiceConfig) Auth() RegistrationServiceAuthConfig {
	return RegistrationServiceAuthConfig{r.cfg.Host.RegistrationService.Auth}
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

func (r RegistrationServiceConfig) Verification() RegistrationServiceVerificationConfig {
	return RegistrationServiceVerificationConfig{c: r.cfg.Host.RegistrationService.Verification, secrets: r.secrets}
}

type RegistrationServiceAnalyticsConfig struct {
	c toolchainv1alpha1.RegistrationServiceAnalyticsConfig
}

func (r RegistrationServiceAnalyticsConfig) WoopraDomain() string {
	return commonconfig.GetString(r.c.WoopraDomain, "")
}

func (r RegistrationServiceAnalyticsConfig) SegmentWriteKey() string {
	return commonconfig.GetString(r.c.SegmentWriteKey, "")
}

type RegistrationServiceAuthConfig struct {
	c toolchainv1alpha1.RegistrationServiceAuthConfig
}

func (r RegistrationServiceAuthConfig) AuthClientLibraryURL() string {
	return commonconfig.GetString(r.c.AuthClientLibraryURL, "https://sso.prod-preview.openshift.io/auth/js/keycloak.js")
}

func (r RegistrationServiceAuthConfig) AuthClientConfigContentType() string {
	return commonconfig.GetString(r.c.AuthClientConfigContentType, "application/json; charset=utf-8")
}

func (r RegistrationServiceAuthConfig) AuthClientConfigRaw() string {
	return commonconfig.GetString(r.c.AuthClientConfigRaw, `{"realm": "toolchain-public","auth-server-url": "https://sso.prod-preview.openshift.io/auth","ssl-required": "none","resource": "crt","clientId": "crt","public-client": true}`)
}

func (r RegistrationServiceAuthConfig) AuthClientPublicKeysURL() string {
	return commonconfig.GetString(r.c.AuthClientPublicKeysURL, "https://sso.prod-preview.openshift.io/auth/realms/toolchain-public/protocol/openid-connect/certs")
}

type RegistrationServiceVerificationConfig struct {
	c       toolchainv1alpha1.RegistrationServiceVerificationConfig
	secrets map[string]map[string]string
}

func (r RegistrationServiceVerificationConfig) registrationServiceSecret(secretKey string) string {
	secret := commonconfig.GetString(r.c.Secret.Ref, "")
	return r.secrets[secret][secretKey]
}

func (r RegistrationServiceVerificationConfig) Enabled() bool {
	return commonconfig.GetBool(r.c.Enabled, false)
}

func (r RegistrationServiceVerificationConfig) DailyLimit() int {
	return commonconfig.GetInt(r.c.DailyLimit, 5)
}

func (r RegistrationServiceVerificationConfig) AttemptsAllowed() int {
	return commonconfig.GetInt(r.c.AttemptsAllowed, 3)
}

func (r RegistrationServiceVerificationConfig) MessageTemplate() string {
	return commonconfig.GetString(r.c.MessageTemplate, "Developer Sandbox for Red Hat OpenShift: Your verification code is %s")
}

func (r RegistrationServiceVerificationConfig) ExcludedEmailDomains() []string {
	excluded := commonconfig.GetString(r.c.ExcludedEmailDomains, "")
	v := strings.FieldsFunc(excluded, func(c rune) bool {
		return c == ','
	})
	return v
}

func (r RegistrationServiceVerificationConfig) CodeExpiresInMin() int {
	return commonconfig.GetInt(r.c.CodeExpiresInMin, 5)
}

func (r RegistrationServiceVerificationConfig) TwilioAccountSID() string {
	key := commonconfig.GetString(r.c.Secret.TwilioAccountSID, "")
	return r.registrationServiceSecret(key)
}

func (r RegistrationServiceVerificationConfig) TwilioAuthToken() string {
	key := commonconfig.GetString(r.c.Secret.TwilioAuthToken, "")
	return r.registrationServiceSecret(key)
}

func (r RegistrationServiceVerificationConfig) TwilioFromNumber() string {
	key := commonconfig.GetString(r.c.Secret.TwilioFromNumber, "")
	return r.registrationServiceSecret(key)
}
