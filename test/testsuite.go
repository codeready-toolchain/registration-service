package test

import (
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/toolchain-common/pkg/test"

	"github.com/stretchr/testify/suite"
)

// UnitTestSuite is the base test suite for unit tests.
type UnitTestSuite struct {
	suite.Suite
	Config *configuration.Registry
}

// SetupSuite sets the suite up and sets testmode.
func (s *UnitTestSuite) SetupSuite() {
	// create logger and registry
	log.Init("registration-service-testing")

	cfg, errs := configuration.CreateEmptyRegistry(test.NewFakeClient(s.T()))
	if errs != nil {
		panic(errs.Error())
	}

	s.Config = cfg

	// set environment to unit-tests
	s.Config.GetViperInstance().Set("environment", configuration.UnitTestsEnvironment)
}

// TearDownSuite tears down the test suite.
func (s *UnitTestSuite) TearDownSuite() {
	// summon the GC!
	s.Config = nil
}
