package log_test

import (
	"fmt"
	"testing"
	"bytes"
	"net/http/httptest"
	"os"
	"strings"
	"errors"

	logger "github.com/codeready-toolchain/registration-service/pkg/log"
	testutils "github.com/codeready-toolchain/registration-service/test"

	"github.com/stretchr/testify/assert"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/suite"
)

type TestLogSuite struct {
	testutils.UnitTestSuite
}

func TestRunLogSuite(t *testing.T) {
	suite.Run(t, &TestLogSuite{testutils.UnitTestSuite{}})
}

func (s *TestLogSuite) TestLogHandler() {
	logger.InitializeLogger("testing")

	s.Run("get logger", func() {
		l := logger.Logger()
		 assert.NotNil(s.T(), l)
	})
	
	s.Run("log info", func() {
		var buf bytes.Buffer
		logger.SetOutput(&buf, true, "logger_tests")
		 defer func() {
			logger.SetOutput(os.Stderr, false, "logger_tests")
		 }()

		 rr := httptest.NewRecorder()
		 ctx, _ := gin.CreateTestContext(rr)
		 ctx.Set("subject", "test")

		 logger.Info(ctx, "info")
		 value := buf.String()
		 assert.True(s.T(), strings.Contains(value, "INFO	logger_tests	info%!(EXTRA []interface {}=[context subject test])"))
	})

	s.Run("log error", func() {
		var buf bytes.Buffer
		logger.SetOutput(&buf, true, "logger_tests")
		 defer func() {
			logger.SetOutput(os.Stderr, false, "logger_tests")
		 }()

		 rr := httptest.NewRecorder()
		 ctx, _ := gin.CreateTestContext(rr)

		 logger.Error(ctx, errors.New("test error"), "error test")
		 value := buf.String()
		 assert.True(s.T(), strings.Contains(value, "ERROR	logger_tests	error test	{\"error\": \"test error\"}"))
	})

	s.Run("log info with http request", func() {
		var buf bytes.Buffer
		logger.SetOutput(&buf, true, "logger_tests")
		 defer func() {
			logger.SetOutput(os.Stderr, false, "logger_tests")
		 }()

		 rr := httptest.NewRecorder()
		 ctx, _ := gin.CreateTestContext(rr)

		 req := httptest.NewRequest("GET", "http://example.com", nil)
		 ctx.Request = req

		 logger.Info(ctx, "info")
		 value := buf.String()
		 fmt.Println(value)
		 assert.True(s.T(), strings.Contains(value, "INFO	logger_tests	info%!(EXTRA []interface {}=[context host: example.com])"))
	})
}
