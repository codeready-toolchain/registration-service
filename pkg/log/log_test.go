package log

import (
	"bytes"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestLogHandler(t *testing.T) {
	var buf bytes.Buffer
	once.Reset()
	Init("logger_tests", &buf)

	t.Run("log info", func(t *testing.T) {
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Set("subject", "test")

		Info(ctx, "test logger with no formatting")
		value := buf.String()
		assert.Contains(t, value, "logger_tests")
		assert.Contains(t, value, "test logger with no formatting")
		assert.Contains(t, value, "\"user_id\":\"test\"")
		assert.Contains(t, value, "info")
		assert.Contains(t, value, "\"timestamp\":")
	})

	t.Run("log infof", func(t *testing.T) {
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Set("subject", "test")

		Infof(ctx, "test %s", "info")
		value := buf.String()
		assert.True(t, strings.Contains(value, "logger_tests"))
		assert.True(t, strings.Contains(value, "test info"))
		assert.True(t, strings.Contains(value, "\"user_id\":\"test\""))
		assert.True(t, strings.Contains(value, "info"))
		assert.True(t, strings.Contains(value, "\"timestamp\":"))
	})

	t.Run("log error", func(t *testing.T) {
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)

		Error(ctx, errors.New("test error"), "test error with no formatting")
		value := buf.String()
		assert.True(t, strings.Contains(value, "logger_tests"))
		assert.True(t, strings.Contains(value, "test error with no formatting"))
		assert.True(t, strings.Contains(value, "\"error\":\"test error\""))
		assert.True(t, strings.Contains(value, "error"))
		assert.True(t, strings.Contains(value, "\"timestamp\":"))
	})

	t.Run("log errorf", func(t *testing.T) {
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)

		Errorf(ctx, errors.New("test error"), "test %s", "info")
		value := buf.String()
		assert.True(t, strings.Contains(value, "logger_tests"))
		assert.True(t, strings.Contains(value, "test info"))
		assert.True(t, strings.Contains(value, "\"error\":\"test error\""))
		assert.True(t, strings.Contains(value, "error"))
		assert.True(t, strings.Contains(value, "\"timestamp\":"))
	})

	t.Run("log infof with http request", func(t *testing.T) {
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)

		req := httptest.NewRequest("GET", "http://example.com/api/v1/health", nil)
		req.Header.Add("Accept", "application/json")
		q := req.URL.Query()
		q.Add("query_key", "query_value")
		req.URL.RawQuery = q.Encode()
		ctx.Request = req

		Infof(ctx, "test %s", "info")
		value := buf.String()
		assert.True(t, strings.Contains(value, "logger_tests"))
		assert.True(t, strings.Contains(value, "test info"))
		assert.True(t, strings.Contains(value, "\"req_url\":\"http://example.com/api/v1/health\""))
		assert.True(t, strings.Contains(value, "info"))
		assert.True(t, strings.Contains(value, "\"timestamp\":"))
		assert.True(t, strings.Contains(value, "\"req_params\":"))
		assert.True(t, strings.Contains(value, "\"query_key\":[\"query_value\"]"))
		assert.True(t, strings.Contains(value, "\"req_headers\":"))
		assert.True(t, strings.Contains(value, "\"Accept\":[\"application/json\"]"))
	})

	t.Run("log infof with http request containing authorization header", func(t *testing.T) {
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)

		data := `{"testing-body":"test"}`
		req := httptest.NewRequest("GET", "http://example.com/api/v1/health", strings.NewReader(data))
		req.Header.Add("Accept", "application/json")
		req.Header.Add("Authorization", "Bearer "+"test-fake-bearer-token")

		q := req.URL.Query()
		q.Add("query_key", "query_value")
		req.URL.RawQuery = q.Encode()
		ctx.Request = req

		Infof(ctx, "test %s", "info")
		value := buf.String()
		fmt.Println(value)
		assert.True(t, strings.Contains(value, "logger_tests"))
		assert.True(t, strings.Contains(value, "test info"))
		assert.Contains(t, value, "\"req_url\":\"http://example.com/api/v1/health\"")
		assert.True(t, strings.Contains(value, "info"))
		assert.True(t, strings.Contains(value, "\"timestamp\":"))
		assert.True(t, strings.Contains(value, "\"req_params\":"))
		assert.True(t, strings.Contains(value, "\"query_key\":[\"query_value\"]"))
		assert.True(t, strings.Contains(value, "\"req_headers\":"))
		assert.True(t, strings.Contains(value, "\"Accept\":[\"application/json\"]"))
		assert.True(t, strings.Contains(value, "\"Authorization\""))
		assert.True(t, strings.Contains(value, "\"*****\""))
		assert.True(t, strings.Contains(value, "\"req_payload\""))
		assert.True(t, strings.Contains(value, "{\\\"testing-body\\\":\\\"test\\\"}"))
	})

	t.Run("log infof withValues", func(t *testing.T) {
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Set("subject", "test")

		WithValues(map[string]interface{}{"testing": "with-values"}).Infof(ctx, "test %s", "info")
		value := buf.String()
		assert.True(t, strings.Contains(value, "logger_tests"))
		assert.True(t, strings.Contains(value, "test info"))
		assert.Contains(t, value, "\"testing\":\"with-values\"")
		assert.Contains(t, value, "\"user_id\":\"test\"")
		assert.True(t, strings.Contains(value, "info"))
		assert.True(t, strings.Contains(value, "\"timestamp\":"))
	})

	t.Run("log infof setOutput when tags is set", func(t *testing.T) {
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Set("subject", "test")

		WithValues(map[string]interface{}{"testing-2": "with-values-2"}).Infof(ctx, "test %s", "info")
		value := buf.String()
		assert.True(t, strings.Contains(value, "logger_tests"))
		assert.True(t, strings.Contains(value, "test info"))
		assert.Contains(t, value, "\"testing\":\"with-values\"")
		assert.Contains(t, value, "\"testing-2\":\"with-values-2\"")
		assert.Contains(t, value, "\"user_id\":\"test\"")
		assert.True(t, strings.Contains(value, "info"))
		assert.True(t, strings.Contains(value, "\"timestamp\":"))
	})
}

/* package log

import (
	"bytes"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/context"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestLogHandler(t *testing.T) {
	once.Reset()
	var buf bytes.Buffer
	Init("logger_tests", &buf)

	t.Run("log info", func(t *testing.T) {
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Set(context.SubKey, "test")
		ctx.Set(context.UsernameKey, "test-user")

		Info(ctx, "test logger with no formatting")
		value := buf.String()
		fmt.Println(value)
		assert.True(t, strings.Contains(value, "logger_tests"))
		assert.True(t, strings.Contains(value, "test logger with no formatting"))
		assert.True(t, strings.Contains(value, "\"user_id\": \"test\""))
		assert.True(t, strings.Contains(value, "\"username\": \"test-user\""))
		assert.True(t, strings.Contains(value, "INFO"))
		assert.True(t, strings.Contains(value, "\"timestamp\":"))
	})

	t.Run("log infof", func(t *testing.T) {
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Set(context.SubKey, "test")

		Infof(ctx, "test %s", "info")
		value := buf.String()
		assert.True(t, strings.Contains(value, "logger_tests"))
		assert.True(t, strings.Contains(value, "test info"))
		assert.True(t, strings.Contains(value, "\"user_id\": \"test\""))
		assert.True(t, strings.Contains(value, "\"username\": \"test-user\""))
		assert.True(t, strings.Contains(value, "INFO"))
		assert.True(t, strings.Contains(value, "\"timestamp\":"))
	})

	t.Run("log error", func(t *testing.T) {
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)

		Error(ctx, errors.New("test error"), "test error with no formatting")
		value := buf.String()
		assert.True(t, strings.Contains(value, "logger_tests"))
		assert.True(t, strings.Contains(value, "test error with no formatting"))
		assert.True(t, strings.Contains(value, "\"error\": \"test error\""))
		assert.True(t, strings.Contains(value, "ERROR"))
		assert.True(t, strings.Contains(value, "\"timestamp\":"))
	})

	t.Run("log errorf", func(t *testing.T) {
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)

		Errorf(ctx, errors.New("test error"), "test %s", "info")
		value := buf.String()
		assert.True(t, strings.Contains(value, "logger_tests"))
		assert.True(t, strings.Contains(value, "test info"))
		assert.True(t, strings.Contains(value, "\"error\": \"test error\""))
		assert.True(t, strings.Contains(value, "ERROR"))
		assert.True(t, strings.Contains(value, "\"timestamp\":"))
	})

	t.Run("log infof with http request", func(t *testing.T) {
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)

		req := httptest.NewRequest("GET", "http://example.com/api/v1/health", nil)
		req.Header.Add("Accept", "application/json")
		q := req.URL.Query()
		q.Add("query_key", "query_value")
		q.Add("token", "secret-token")
		req.URL.RawQuery = q.Encode()
		ctx.Request = req

		Infof(ctx, "test %s", "info")
		value := buf.String()
		assert.True(t, strings.Contains(value, "logger_tests"))
		assert.True(t, strings.Contains(value, "test info"))
		assert.True(t, strings.Contains(value, "\"req_url\": \"http://example.com/api/v1/health\""))
		assert.True(t, strings.Contains(value, "INFO"))
		assert.True(t, strings.Contains(value, "\"timestamp\":"))
		assert.True(t, strings.Contains(value, "\"req_params\":"))
		assert.True(t, strings.Contains(value, "\"query_key\":[\"query_value\"]"))
		assert.True(t, strings.Contains(value, "\"token\":[\"*****\"]"))
		assert.True(t, strings.Contains(value, "\"req_headers\":"))
		assert.True(t, strings.Contains(value, "\"Accept\":[\"application/json\"]"))
	})

	t.Run("log infof with http request containing authorization header", func(t *testing.T) {
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)

		data := `{"testing-body":"test"}`
		req := httptest.NewRequest("GET", "http://example.com/api/v1/health", strings.NewReader(data))
		req.Header.Add("Accept", "application/json")
		req.Header.Add("Authorization", "Bearer "+"test-fake-bearer-token")

		q := req.URL.Query()
		q.Add("query_key", "query_value")
		req.URL.RawQuery = q.Encode()
		ctx.Request = req

		Infof(ctx, "test %s", "info")
		value := buf.String()
		fmt.Println(value)
		assert.True(t, strings.Contains(value, "logger_tests"))
		assert.True(t, strings.Contains(value, "test info"))
		assert.True(t, strings.Contains(value, "\"req_url\": \"http://example.com/api/v1/health\""))
		assert.True(t, strings.Contains(value, "INFO"))
		assert.True(t, strings.Contains(value, "\"timestamp\":"))
		assert.True(t, strings.Contains(value, "\"req_params\":"))
		assert.True(t, strings.Contains(value, "\"query_key\":[\"query_value\"]"))
		assert.True(t, strings.Contains(value, "\"req_headers\":"))
		assert.True(t, strings.Contains(value, "\"Accept\":[\"application/json\"]"))
		assert.True(t, strings.Contains(value, "\"Authorization\""))
		assert.True(t, strings.Contains(value, "\"*****\""))
		assert.True(t, strings.Contains(value, "\"req_payload\""))
		assert.True(t, strings.Contains(value, "{\\\"testing-body\\\":\\\"test\\\"}"))
	})

	t.Run("log infof withValues", func(t *testing.T) {
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Set(context.SubKey, "test")

		m := make(map[string]interface{})
		m["testing"] = "with-values"

		WithValues(m).Infof(ctx, "test %s", "info")
		value := buf.String()
		assert.True(t, strings.Contains(value, "logger_tests"))
		assert.True(t, strings.Contains(value, "test info"))
		assert.True(t, strings.Contains(value, "\"testing\": \"with-values\""))
		assert.True(t, strings.Contains(value, "\"user_id\": \"test\""))
		assert.True(t, strings.Contains(value, "\"username\": \"test-user\""))
		assert.True(t, strings.Contains(value, "INFO"))
		assert.True(t, strings.Contains(value, "\"timestamp\":"))
	})

	t.Run("log infof setOutput when tags is set", func(t *testing.T) {
		m := make(map[string]interface{})
		m["testing-2"] = "with-values-2"

		logr := WithValues(m)

		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Set(context.SubKey, "test")

		logr.Infof(ctx, "test %s", "info")
		value := buf.String()
		assert.True(t, strings.Contains(value, "logger_tests"))
		assert.True(t, strings.Contains(value, "test info"))
		assert.True(t, strings.Contains(value, "\"testing\": \"with-values\""))
		assert.True(t, strings.Contains(value, "\"testing-2\": \"with-values-2\""))
		assert.True(t, strings.Contains(value, "\"user_id\": \"test\""))
		assert.True(t, strings.Contains(value, "\"username\": \"test-user\""))
		assert.True(t, strings.Contains(value, "INFO"))
		assert.True(t, strings.Contains(value, "\"timestamp\":"))
	})
}
*/
