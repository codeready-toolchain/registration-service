package configuration_test

import (
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/test"
	commonconfig "github.com/codeready-toolchain/toolchain-common/pkg/configuration"
	testconfig "github.com/codeready-toolchain/toolchain-common/pkg/test/config"
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

func (s *TestConfigurationSuite) TestSegmentWriteKey() {
	s.Run("unit-test environment (default)", func() {
		require.True(s.T(), configuration.IsTestingMode())
	})

	s.Run("prod environment", func() {
		s.SetConfig(testconfig.RegistrationService().Environment(configuration.DefaultEnvironment))
		require.False(s.T(), configuration.IsTestingMode())
	})
}

func TestRegistrationService(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		cfg := commonconfig.NewToolchainConfigObjWithReset(t)
		regServiceCfg := configuration.NewRegistrationServiceConfig(cfg, map[string]map[string]string{})

		assert.Equal(t, "prod", regServiceCfg.Environment())
		assert.Equal(t, "info", regServiceCfg.LogLevel())
		assert.Equal(t, "toolchain-host-operator", regServiceCfg.Namespace())
		assert.Equal(t, "https://registration.crt-placeholder.com", regServiceCfg.RegistrationServiceURL())
		assert.Empty(t, regServiceCfg.Analytics().SegmentWriteKey())
		assert.Empty(t, regServiceCfg.Analytics().WoopraDomain())
		assert.Equal(t, "https://sso.prod-preview.openshift.io/auth/js/keycloak.js", regServiceCfg.Auth().AuthClientLibraryURL())
		assert.Equal(t, "application/json; charset=utf-8", regServiceCfg.Auth().AuthClientConfigContentType())
		assert.Equal(t, `{"realm": "toolchain-public","auth-server-url": "https://sso.prod-preview.openshift.io/auth","ssl-required": "none","resource": "crt","clientId": "crt","public-client": true}`,
			regServiceCfg.Auth().AuthClientConfigRaw())
		assert.Equal(t, "https://sso.prod-preview.openshift.io/auth/realms/toolchain-public/protocol/openid-connect/certs", regServiceCfg.Auth().AuthClientPublicKeysURL())
		assert.False(t, regServiceCfg.Verification().Enabled())
		assert.Equal(t, 5, regServiceCfg.Verification().DailyLimit())
		assert.Equal(t, 3, regServiceCfg.Verification().AttemptsAllowed())
		assert.Equal(t, "Developer Sandbox for Red Hat OpenShift: Your verification code is %s", regServiceCfg.Verification().MessageTemplate())
		assert.Empty(t, regServiceCfg.Verification().ExcludedEmailDomains())
		assert.Equal(t, 5, regServiceCfg.Verification().CodeExpiresInMin())
		assert.Empty(t, regServiceCfg.Verification().TwilioAccountSID())
		assert.Empty(t, regServiceCfg.Verification().TwilioAuthToken())
		assert.Empty(t, regServiceCfg.Verification().TwilioFromNumber())
	})
	t.Run("non-default", func(t *testing.T) {
		cfg := commonconfig.NewToolchainConfigObjWithReset(t, testconfig.RegistrationService().
			Environment("e2e-tests").
			LogLevel("debug").
			Namespace("another-namespace").
			RegistrationServiceURL("www.crtregservice.com").
			Analytics().SegmentWriteKey("keyabc").
			Analytics().WoopraDomain("woopra.com").
			Auth().AuthClientLibraryURL("https://sso.openshift.com/auth/js/keycloak.js").
			Auth().AuthClientConfigContentType("application/xml").
			Auth().AuthClientConfigRaw(`{"realm": "toolchain-private"}`).
			Auth().AuthClientPublicKeysURL("https://sso.openshift.com/certs").
			Verification().Enabled(true).
			Verification().DailyLimit(15).
			Verification().AttemptsAllowed(13).
			Verification().MessageTemplate("Developer Sandbox verification code: %s").
			Verification().ExcludedEmailDomains("redhat.com,ibm.com").
			Verification().CodeExpiresInMin(151).
			Verification().Secret().Ref("verification-secrets").TwilioAccountSID("twilio.sid").TwilioAuthToken("twilio.token").TwilioFromNumber("twilio.fromnumber"))

		verificationSecretValues := make(map[string]string)
		verificationSecretValues["twilio.sid"] = "def"
		verificationSecretValues["twilio.token"] = "ghi"
		verificationSecretValues["twilio.fromnumber"] = "jkl"
		secrets := make(map[string]map[string]string)
		secrets["verification-secrets"] = verificationSecretValues

		regServiceCfg := configuration.NewRegistrationServiceConfig(cfg, secrets)

		assert.Equal(t, "e2e-tests", regServiceCfg.Environment())
		assert.Equal(t, "debug", regServiceCfg.LogLevel())
		assert.Equal(t, "another-namespace", regServiceCfg.Namespace())
		assert.Equal(t, "www.crtregservice.com", regServiceCfg.RegistrationServiceURL())
		assert.Equal(t, "keyabc", regServiceCfg.Analytics().SegmentWriteKey())
		assert.Equal(t, "woopra.com", regServiceCfg.Analytics().WoopraDomain())
		assert.Equal(t, "https://sso.openshift.com/auth/js/keycloak.js", regServiceCfg.Auth().AuthClientLibraryURL())
		assert.Equal(t, "application/xml", regServiceCfg.Auth().AuthClientConfigContentType())
		assert.Equal(t, `{"realm": "toolchain-private"}`, regServiceCfg.Auth().AuthClientConfigRaw())
		assert.Equal(t, "https://sso.openshift.com/certs", regServiceCfg.Auth().AuthClientPublicKeysURL())

		assert.True(t, regServiceCfg.Verification().Enabled())
		assert.Equal(t, 15, regServiceCfg.Verification().DailyLimit())
		assert.Equal(t, 13, regServiceCfg.Verification().AttemptsAllowed())
		assert.Equal(t, "Developer Sandbox verification code: %s", regServiceCfg.Verification().MessageTemplate())
		assert.Equal(t, []string{"redhat.com", "ibm.com"}, regServiceCfg.Verification().ExcludedEmailDomains())
		assert.Equal(t, 151, regServiceCfg.Verification().CodeExpiresInMin())
		assert.Equal(t, "def", regServiceCfg.Verification().TwilioAccountSID())
		assert.Equal(t, "ghi", regServiceCfg.Verification().TwilioAuthToken())
		assert.Equal(t, "jkl", regServiceCfg.Verification().TwilioFromNumber())
	})
}
