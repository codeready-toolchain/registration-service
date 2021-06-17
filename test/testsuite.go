package test

import (
	"github.com/codeready-toolchain/registration-service/pkg/application/service/factory"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/test/fake"
	commonconfig "github.com/codeready-toolchain/toolchain-common/pkg/configuration"
	"github.com/codeready-toolchain/toolchain-common/pkg/test"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// UnitTestSuite is the base test suite for unit tests.
type UnitTestSuite struct {
	suite.Suite
	config                     *configuration.ViperConfig
	Application                *fake.MockableApplication
	FakeUserSignupClient       *fake.FakeUserSignupClient
	FakeMasterUserRecordClient *fake.FakeMasterUserRecordClient
	FakeBannedUserClient       *fake.FakeBannedUserClient
	FakeToolchainStatusClient  *fake.FakeToolchainStatusClient
	factoryOptions             []factory.Option
	configOverride             configuration.Configuration
}

func (s *UnitTestSuite) Config() configuration.Configuration {
	if s.configOverride != nil {
		return s.configOverride
	}
	return s.config
}

// ViperConfig is purely here as a bypass for some tests that require access to the viper configuration directly
func (s *UnitTestSuite) ViperConfig() *configuration.ViperConfig {
	return s.config
}

// SetupSuite sets the suite up and sets testmode.
func (s *UnitTestSuite) SetupSuite() {
	// create logger and registry
	log.Init("registration-service-testing")
}

func (s *UnitTestSuite) SetupTest() {
	s.factoryOptions = nil
	s.configOverride = nil
	s.SetupDefaultApplication()
}

func (s *UnitTestSuite) SetupDefaultApplication() {
	s.config = s.DefaultConfig()
	s.FakeUserSignupClient = fake.NewFakeUserSignupClient(s.T(), s.config.GetNamespace())
	s.FakeMasterUserRecordClient = fake.NewFakeMasterUserRecordClient(s.T(), s.config.GetNamespace())
	s.FakeBannedUserClient = fake.NewFakeBannedUserClient(s.T(), s.config.GetNamespace())
	s.FakeToolchainStatusClient = fake.NewFakeToolchainStatusClient(s.T(), s.config.GetNamespace())
	s.Application = fake.NewMockableApplication(s.config, s, s.factoryOptions...)
}

func (s *UnitTestSuite) DefaultConfig() *configuration.ViperConfig {
	restore := test.SetEnvVarAndRestore(s.T(), commonconfig.WatchNamespaceEnvVar, "toolchain-host-operator")
	defer restore()

	cfg, err := configuration.LoadConfig(test.NewFakeClient(s.T()))
	require.NoError(s.T(), err)
	cfg.GetViperInstance().Set("environment", configuration.UnitTestsEnvironment)
	return cfg
}

func (s *UnitTestSuite) OverrideConfig(config configuration.Configuration) {
	s.configOverride = config
	s.FakeUserSignupClient = fake.NewFakeUserSignupClient(s.T(), config.GetNamespace())
	s.FakeMasterUserRecordClient = fake.NewFakeMasterUserRecordClient(s.T(), config.GetNamespace())
	s.FakeBannedUserClient = fake.NewFakeBannedUserClient(s.T(), config.GetNamespace())
	s.FakeToolchainStatusClient = fake.NewFakeToolchainStatusClient(s.T(), config.GetNamespace())
	s.Application = fake.NewMockableApplication(config, s, s.factoryOptions...)
}

func (s *UnitTestSuite) WithFactoryOption(opt factory.Option) {
	s.factoryOptions = append(s.factoryOptions, opt)
}

// TearDownSuite tears down the test suite.
func (s *UnitTestSuite) TearDownSuite() {
	// summon the GC!
	s.config = nil
	s.Application = nil
	s.FakeUserSignupClient = nil
	s.FakeMasterUserRecordClient = nil
	s.FakeBannedUserClient = nil
	s.FakeToolchainStatusClient = nil
}

func (s *UnitTestSuite) V1Alpha1() kubeclient.V1Alpha1 {
	return s
}

func (s *UnitTestSuite) UserSignups() kubeclient.UserSignupInterface {
	return s.FakeUserSignupClient
}

func (s *UnitTestSuite) MasterUserRecords() kubeclient.MasterUserRecordInterface {
	return s.FakeMasterUserRecordClient
}

func (s *UnitTestSuite) BannedUsers() kubeclient.BannedUserInterface {
	return s.FakeBannedUserClient
}

func (s *UnitTestSuite) ToolchainStatuses() kubeclient.ToolchainStatusInterface {
	return s.FakeToolchainStatusClient
}
