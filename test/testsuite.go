package test

import (
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/log"

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

	s.Config = configuration.CreateEmptyRegistry()

	// set environment to unit-tests
	s.Config.GetViperInstance().Set("environment", configuration.UnitTestsEnvironment)
}

// TearDownSuite tears down the test suite.
func (s *UnitTestSuite) TearDownSuite() {
	// summon the GC!
	s.Config = nil
}
