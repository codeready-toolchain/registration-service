package handlers

import (
	"github.com/codeready-toolchain/registration-service/test"
	"github.com/stretchr/testify/suite"
	"testing"
)

type TestMetricsSuite struct {
	test.UnitTestSuite
}

func TestRunMetricsSuite(t *testing.T) {
	suite.Run(t, &TestMetricsSuite{test.UnitTestSuite{}})
}

