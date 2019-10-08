package log_test

import (
	"bytes"
	"errors"
	"fmt"
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

	s.Run("get logger", func() {
		l := logger.GetLogger()
		assert.NotNil(s.T(), lgr)
		assert.NotNil(s.T(), l)
	})

	s.Run("log info", func() {
		var buf bytes.Buffer
		lgr.SetOutput(&buf, true)
		defer func() {
			lgr.SetOutput(os.Stderr, false)
		}()

		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Set("subject", "test")

		lgr.Infof(ctx, "test %s", "info")
		value := buf.String()
		assert.True(s.T(), strings.Contains(value, "INFO	logger_tests	test info	{\"user_id\": \"test\"}"))
	})

	s.Run("log error", func() {
		var buf bytes.Buffer
		lgr.SetOutput(&buf, true)
		defer func() {
			lgr.SetOutput(os.Stderr, false)
		}()

		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)

		lgr.Errorf(ctx, errors.New("test error"),  "test %s", "info")
		value := buf.String()
		assert.True(s.T(), strings.Contains(value, "ERROR	logger_tests	test info	{\"error\": \"test error\"}"))
	})

	s.Run("log info with http request", func() {
		var buf bytes.Buffer
		lgr.SetOutput(&buf, true)
		defer func() {
			lgr.SetOutput(os.Stderr, false)
		}()

		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)

		req := httptest.NewRequest("GET", "http://example.com/api/v1/health", nil)
		ctx.Request = req

		lgr.Infof(ctx, "test %s", "info")
		value := buf.String()
		assert.True(s.T(), strings.Contains(value, "INFO	logger_tests	test info	{\"req_url\": \"http://example.com/api/v1/health\"}"))
	})

	s.Run("log info withValues", func() {
		var buf bytes.Buffer
		lgr.SetOutput(&buf, true)
		defer func() {
			lgr.SetOutput(os.Stderr, false)
		}()

		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Set("subject", "test")

		lgr.WithValues("tina", "kurian").Infof(ctx, "test %s", "info")
		value := buf.String()
		fmt.Println(value)
		assert.True(s.T(), strings.Contains(value, "INFO	logger_tests	test info	{\"user_id\": \"test\"}"))
	})

	// s.Run("setOutput when tags is set", func() {
	// 	var buf bytes.Buffer
	// 	lgr.WithValues("tina", "kurian")
	// 	lgr.SetOutput(&buf, true)
	// 	defer func() {
	// 		lgr.SetOutput(os.Stderr, false)
	// 	}()

	// 	rr := httptest.NewRecorder()
	// 	ctx, _ := gin.CreateTestContext(rr)
	// 	ctx.Set("subject", "test")

	// 	lgr.Infof(ctx, "test %s", "info")
	// 	value := buf.String()
	// 	fmt.Println(value)
	// 	assert.True(s.T(), strings.Contains(value, "INFO	logger_tests	test info	{\"user_id\": \"test\"}"))
	// })
}
