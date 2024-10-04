package test

import (
	"context"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/application/service/factory"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/test/fake"
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
	Application    *fake.MockableApplication
	ConfigClient   *test.FakeClient
	factoryOptions []factory.Option
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
	s.Application = fake.NewMockableApplication(s.factoryOptions...)
}

func (s *UnitTestSuite) OverrideApplicationDefault(opts ...testconfig.ToolchainConfigOption) {
	s.SetConfig(opts...)
	s.Application = fake.NewMockableApplication(s.factoryOptions...)
}

func (s *UnitTestSuite) SetConfig(opts ...testconfig.ToolchainConfigOption) configuration.RegistrationServiceConfig {

	current := &toolchainv1alpha1.ToolchainConfig{}
	err := s.ConfigClient.Get(context.TODO(), types.NamespacedName{Name: "config", Namespace: test.HostOperatorNs}, current)

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

	// set client & get the config
	configuration.SetClient(s.ConfigClient)
	return configuration.GetRegistrationServiceConfig()
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
	// set client
	configuration.SetClient(s.ConfigClient)
}

func (s *UnitTestSuite) DefaultConfig() configuration.RegistrationServiceConfig {
	// use a new configuration client to fully reset configuration
	s.ConfigClient = test.NewFakeClient(s.T())
	commonconfig.ResetCache()
	obj := testconfig.NewToolchainConfigObj(s.T(), testconfig.RegistrationService().Environment("unit-tests"))
	err := s.ConfigClient.Create(context.TODO(), obj)
	require.NoError(s.T(), err)
	// set client & get the config
	configuration.SetClient(s.ConfigClient)
	return configuration.GetRegistrationServiceConfig()
}

// TearDownSuite tears down the test suite.
func (s *UnitTestSuite) TearDownSuite() {
	// summon the GC!
	commonconfig.ResetCache()
	s.Application = nil
}
