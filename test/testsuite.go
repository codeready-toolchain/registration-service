package test

import (
	"context"
	"os"

	"github.com/codeready-toolchain/registration-service/pkg/application/service/factory"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/kubeclient"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/test/fake"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	commonconfig "github.com/codeready-toolchain/toolchain-common/pkg/configuration"
	"github.com/codeready-toolchain/toolchain-common/pkg/test"
	testconfig "github.com/codeready-toolchain/toolchain-common/pkg/test/config"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// UnitTestSuite is the base test suite for unit tests.
type UnitTestSuite struct {
	suite.Suite
	Application                *fake.MockableApplication
	ConfigClient               *test.FakeClient
	FakeUserSignupClient       *fake.FakeUserSignupClient
	FakeMasterUserRecordClient *fake.FakeMasterUserRecordClient
	FakeBannedUserClient       *fake.FakeBannedUserClient
	FakeToolchainEventClient   *fake.FakeToolchainEventClient
	FakeToolchainStatusClient  *fake.FakeToolchainStatusClient
	factoryOptions             []factory.Option
}

// SetupSuite sets the suite up and sets testmode.
func (s *UnitTestSuite) SetupSuite() {
	// create logger and registry
	log.Init("registration-service-testing")
	test.SetEnvVarAndRestore(s.T(), commonconfig.WatchNamespaceEnvVar, test.HostOperatorNs)
}

func (s *UnitTestSuite) SetupTest() {
	s.factoryOptions = nil
	s.SetupDefaultApplication()
}

func (s *UnitTestSuite) SetupDefaultApplication() {
	// initialize the toolchainconfig cache
	s.DefaultConfig()
	s.FakeUserSignupClient = fake.NewFakeUserSignupClient(s.T(), configuration.Namespace())
	s.FakeMasterUserRecordClient = fake.NewFakeMasterUserRecordClient(s.T(), configuration.Namespace())
	s.FakeBannedUserClient = fake.NewFakeBannedUserClient(s.T(), configuration.Namespace())
	s.FakeToolchainStatusClient = fake.NewFakeToolchainStatusClient(s.T(), configuration.Namespace())
	s.Application = fake.NewMockableApplication(s, s.factoryOptions...)
}

func (s *UnitTestSuite) OverrideApplicationDefault(opts ...testconfig.ToolchainConfigOption) {
	s.SetConfig(opts...)
	s.FakeUserSignupClient = fake.NewFakeUserSignupClient(s.T(), configuration.Namespace())
	s.FakeMasterUserRecordClient = fake.NewFakeMasterUserRecordClient(s.T(), configuration.Namespace())
	s.FakeBannedUserClient = fake.NewFakeBannedUserClient(s.T(), configuration.Namespace())
	s.FakeToolchainStatusClient = fake.NewFakeToolchainStatusClient(s.T(), configuration.Namespace())
	s.Application = fake.NewMockableApplication(s, s.factoryOptions...)
}

func (s *UnitTestSuite) SetConfig(opts ...testconfig.ToolchainConfigOption) configuration.RegistrationServiceConfig {

	namespace, found := os.LookupEnv(commonconfig.WatchNamespaceEnvVar)
	require.Truef(s.T(), found, "%s env var is not", commonconfig.WatchNamespaceEnvVar)

	current := &toolchainv1alpha1.ToolchainConfig{}
	err := s.ConfigClient.Get(context.TODO(), types.NamespacedName{Name: "config", Namespace: namespace}, current)

	if err == nil {
		err = s.ConfigClient.Delete(context.TODO(), current)
		require.NoError(s.T(), err)
	} else {
		// only proceed to create the toolchainconfig if it was not found
		require.True(s.T(), errors.IsNotFound(err), "unexpected error", err.Error())
	}

	newcfg := testconfig.NewToolchainConfigObj(s.T(), opts...)
	err = s.ConfigClient.Create(context.TODO(), newcfg)
	require.NoError(s.T(), err)

	// update config cache
	cfg, err := configuration.ForceLoadRegistrationServiceConfig(s.ConfigClient)
	require.NoError(s.T(), err)
	return cfg
}

func (s *UnitTestSuite) SetSecret(secret *corev1.Secret) {
	sec := &corev1.Secret{}
	err := s.ConfigClient.Get(context.TODO(), types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, sec)

	if err == nil {
		err = s.ConfigClient.Delete(context.TODO(), sec)
		require.NoError(s.T(), err)
	}

	require.True(s.T(), errors.IsNotFound(err), "unexpected error")
	err = s.ConfigClient.Create(context.TODO(), secret)
	require.NoError(s.T(), err)
	// update config cache
	cfg, err := configuration.ForceLoadRegistrationServiceConfig(s.ConfigClient)
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), cfg)
}

func (s *UnitTestSuite) DefaultConfig() configuration.RegistrationServiceConfig {
	// use a new configuration client to fully reset configuration
	s.ConfigClient = test.NewFakeClient(s.T())
	commonconfig.ResetCache()
	obj := testconfig.NewToolchainConfigObj(s.T(), testconfig.RegistrationService().Environment("unit-tests"))
	err := s.ConfigClient.Create(context.TODO(), obj)
	require.NoError(s.T(), err)
	cfg, err := configuration.ForceLoadRegistrationServiceConfig(s.ConfigClient)
	require.NoError(s.T(), err)
	return cfg
}

func (s *UnitTestSuite) WithFactoryOption(opt factory.Option) {
	s.factoryOptions = append(s.factoryOptions, opt)
}

// TearDownSuite tears down the test suite.
func (s *UnitTestSuite) TearDownSuite() {
	// summon the GC!
	commonconfig.ResetCache()
	s.Application = nil
	s.FakeUserSignupClient = nil
	s.FakeMasterUserRecordClient = nil
	s.FakeBannedUserClient = nil
	s.FakeToolchainEventClient = nil
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

func (s *UnitTestSuite) ToolchainEvents() kubeclient.ToolchainEventInterface {
	return s.FakeToolchainEventClient
}

func (s *UnitTestSuite) ToolchainStatuses() kubeclient.ToolchainStatusInterface {
	return s.FakeToolchainStatusClient
}
