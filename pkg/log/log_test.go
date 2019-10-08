package log_test

import (
	"bytes"
	"errors"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	logger "github.com/codeready-toolchain/registration-service/pkg/log"
	testutils "github.com/codeready-toolchain/registration-service/test"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type TestLogSuite struct {
	testutils.UnitTestSuite
}

func TestRunLogSuite(t *testing.T) {
	suite.Run(t, &TestLogSuite{testutils.UnitTestSuite{}})
}

func (s *TestLogSuite) TestLogHandler() {
	lgr := logger.InitializeLogger("logger_tests")
	var buf bytes.Buffer
	lgr.SetOutput(&buf, true)
	defer func() {
		lgr.SetOutput(os.Stderr, false)
	}()

	s.Run("get logger", func() {
		l := logger.GetLogger()
		assert.NotNil(s.T(), lgr)
		assert.NotNil(s.T(), l)
	})

	s.Run("log info", func() {
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Set("subject", "test")

		lgr.Info(ctx, "test logger with no formatting")
		value := buf.String()
		assert.True(s.T(), strings.Contains(value, "logger_tests"))
		assert.True(s.T(), strings.Contains(value, "test logger with no formatting"))
		assert.True(s.T(), strings.Contains(value, "\"user_id\": \"test\"}"))
		assert.True(s.T(), strings.Contains(value, "INFO"))
	})

	s.Run("log infof", func() {
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Set("subject", "test")

		lgr.Infof(ctx, "test %s", "info")
		value := buf.String()
		assert.True(s.T(), strings.Contains(value, "logger_tests"))
		assert.True(s.T(), strings.Contains(value, "test info"))
		assert.True(s.T(), strings.Contains(value, "\"user_id\": \"test\"}"))
		assert.True(s.T(), strings.Contains(value, "INFO"))
	})

	s.Run("log error", func() {
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)

		lgr.Error(ctx, errors.New("test error"), "test error with no formatting")
		value := buf.String()
		assert.True(s.T(), strings.Contains(value, "logger_tests"))
		assert.True(s.T(), strings.Contains(value, "test error with no formatting"))
		assert.True(s.T(), strings.Contains(value, "\"error\": \"test error\"}"))
		assert.True(s.T(), strings.Contains(value, "ERROR"))
	})

	s.Run("log errorf", func() {
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)

		lgr.Errorf(ctx, errors.New("test error"), "test %s", "info")
		value := buf.String()
		assert.True(s.T(), strings.Contains(value, "logger_tests"))
		assert.True(s.T(), strings.Contains(value, "test info"))
		assert.True(s.T(), strings.Contains(value, "\"error\": \"test error\"}"))
		assert.True(s.T(), strings.Contains(value, "ERROR"))
	})

	s.Run("log infof with http request", func() {
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)

		req := httptest.NewRequest("GET", "http://example.com/api/v1/health", nil)
		ctx.Request = req

		lgr.Infof(ctx, "test %s", "info")
		value := buf.String()
		assert.True(s.T(), strings.Contains(value, "logger_tests"))
		assert.True(s.T(), strings.Contains(value, "test info"))
		assert.True(s.T(), strings.Contains(value, "\"req_url\": \"http://example.com/api/v1/health\"}"))
		assert.True(s.T(), strings.Contains(value, "INFO"))
	})

	s.Run("log infof withValues", func() {
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Set("subject", "test")

		lgr.WithValues("testing", "with-values").Infof(ctx, "test %s", "info")
		value := buf.String()
		assert.True(s.T(), strings.Contains(value, "logger_tests"))
		assert.True(s.T(), strings.Contains(value, "test info"))
		assert.True(s.T(), strings.Contains(value, "\"testing\": \"with-values\""))
		assert.True(s.T(), strings.Contains(value, "\"user_id\": \"test\""))
		assert.True(s.T(), strings.Contains(value, "INFO"))
	})

	s.Run("log infof setOutput when tags is set", func() {
		lgr.WithValues("testing-2", "with-values-2")

		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Set("subject", "test")

		lgr.Infof(ctx, "test %s", "info")
		value := buf.String()
		assert.True(s.T(), strings.Contains(value, "logger_tests"))
		assert.True(s.T(), strings.Contains(value, "test info"))
		assert.True(s.T(), strings.Contains(value, "\"testing\": \"with-values\""))
		assert.True(s.T(), strings.Contains(value, "\"testing-2\": \"with-values-2\""))
		assert.True(s.T(), strings.Contains(value, "\"user_id\": \"test\""))
		assert.True(s.T(), strings.Contains(value, "INFO"))
	})
}
