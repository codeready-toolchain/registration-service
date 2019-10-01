package log_test

import (
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/log"
	testutils "github.com/codeready-toolchain/registration-service/test"
	"github.com/stretchr/testify/suite"
)

type TestLogSuite struct {
	testutils.UnitTestSuite
}

func TestRunLogSuite(t *testing.T) {
	suite.Run(t, &TestLogSuite{testutils.UnitTestSuite{}})
}

func (s *TestLogSuite) TestLogHandler() {
	log.Println(nil, "tina")
}
