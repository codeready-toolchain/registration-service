package test

import (
	"github.com/codeready-toolchain/registration-service/pkg/application/service/factory"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"

	"github.com/codeready-toolchain/registration-service/test/fake"
	"github.com/codeready-toolchain/toolchain-common/pkg/test"
	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/suite"
)

// UnitTestSuite is the base test suite for unit tests.
type UnitTestSuite struct {
	suite.Suite
	Config                     *configuration.ViperConfig
	Application                *fake.MockableApplication
	FakeUserSignupClient       *fake.FakeUserSignupClient
	FakeMasterUserRecordClient *fake.FakeMasterUserRecordClient
	FakeBannedUserClient       *fake.FakeBannedUserClient
	factoryOptions             []factory.Option
}

// SetupSuite sets the suite up and sets testmode.
func (s *UnitTestSuite) SetupSuite() {
	// create logger and registry
	log.Init("registration-service-testing")
}

func (s *UnitTestSuite) SetupTest() {
	s.factoryOptions = nil
	s.SetupDefaultApplication()
}

func (s *UnitTestSuite) SetupDefaultApplication() {
	s.Config = s.DefaultConfig()
	s.FakeUserSignupClient = fake.NewFakeUserSignupClient(s.Config.GetNamespace())
	s.FakeMasterUserRecordClient = fake.NewFakeMasterUserRecordClient(s.Config.GetNamespace())
	s.FakeBannedUserClient = fake.NewFakeBannedUserClient(s.Config.GetNamespace())
	s.Application = fake.NewMockableApplication(s.Config, s, s.factoryOptions...)
}

func (s *UnitTestSuite) DefaultConfig() *configuration.ViperConfig {
	restore := test.SetEnvVarAndRestore(s.T(), k8sutil.WatchNamespaceEnvVar, "toolchain-host-operator")
	defer restore()

	cfg, err := configuration.CreateEmptyConfig(test.NewFakeClient(s.T()))
	require.NoError(s.T(), err)
	cfg.GetViperInstance().Set("environment", configuration.UnitTestsEnvironment)
	return cfg
}

func (s *UnitTestSuite) SetupApplication(config configuration.Configuration) {
	s.FakeUserSignupClient = fake.NewFakeUserSignupClient(config.GetNamespace())
	s.FakeMasterUserRecordClient = fake.NewFakeMasterUserRecordClient(config.GetNamespace())
	s.FakeBannedUserClient = fake.NewFakeBannedUserClient(config.GetNamespace())
	s.Application = fake.NewMockableApplication(config, s, s.factoryOptions...)
}

func (s *UnitTestSuite) WithFactoryOption(opt factory.Option) {
	s.factoryOptions = append(s.factoryOptions, opt)
}

// TearDownSuite tears down the test suite.
func (s *UnitTestSuite) TearDownSuite() {
	// summon the GC!
	s.Config = nil
	s.Application = nil
	s.FakeUserSignupClient = nil
	s.FakeMasterUserRecordClient = nil
	s.FakeBannedUserClient = nil
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
