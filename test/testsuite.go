package testutils

import (
	"log"
	"os"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/stretchr/testify/suite"
)

// UnitTestSuite is the base test suite for unit tests.
type UnitTestSuite struct {
	suite.Suite
	ConfigRegistry *configuration.Registry
	Logger         *log.Logger
}

// SetupSuite sets the suite up and sets testmode.
func (s *UnitTestSuite) SetupSuite() {
	// create logger and registry
	s.Logger = log.New(os.Stderr, "", 0)
	s.ConfigRegistry = configuration.CreateEmptyRegistry()

	// set the config for testing mode
	s.ConfigRegistry.GetViperInstance().Set("testingmode", true)
	//assert.True(t, s.ConfigRegistry.IsTestingMode(), "testing mode not set correctly to true")
}

// TearDownSuite tears down the test suite.
func (s *UnitTestSuite) TearDownSuite() {
	// summon the GC!
	s.ConfigRegistry = nil
	s.Logger = nil
}
