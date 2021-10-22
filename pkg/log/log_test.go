package log

import (
	"bytes"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestLog(t *testing.T) {
	var buf bytes.Buffer
	once.Reset()
	Init("logger_tests", func(o *zap.Options) {
		o.DestWriter = &buf
	})

	t.Run("log info", func(t *testing.T) {
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Set("subject", "test")
		ctx.Set("username", "usernametest")

		Info(ctx, "test logger with no formatting")
		value := buf.String()
		assert.Contains(t, value, `"logger":"logger_tests"`)
		assert.Contains(t, value, `"msg":"test logger with no formatting"`)
		assert.Contains(t, value, `"user_id":"test"`)
		assert.Contains(t, value, `"username":"usernametest"`)
		assert.Contains(t, value, `"level":"info"`)
		assert.Contains(t, value, `"timestamp":"`)
		assert.Contains(t, value, fmt.Sprintf(`"commit":"%s"`, configuration.Commit))
	})

	t.Run("log infof", func(t *testing.T) {
		buf.Reset()
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Set(context.SubKey, "test")
		ctx.Set(context.UsernameKey, "usernametest")

		Infof(ctx, "test %s", "info")
		value := buf.String()
		assert.Contains(t, value, `"logger":"logger_tests"`)
		assert.Contains(t, value, `"msg":"test info"`)
		assert.Contains(t, value, `"user_id":"test"`)
		assert.Contains(t, value, `"username":"usernametest"`)
		assert.Contains(t, value, `"level":"info"`)
		assert.Contains(t, value, `"timestamp":"`)
	})

	t.Run("log infof with no arguments", func(t *testing.T) {
		buf.Reset()
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)

		Infof(ctx, "test")
		value := buf.String()
		assert.Contains(t, value, `"logger":"logger_tests"`)
		assert.Contains(t, value, `"msg":"test"`)
	})

	t.Run("log error", func(t *testing.T) {
		buf.Reset()
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)

		Error(ctx, errors.New("test error"), "test error with no formatting")
		value := buf.String()
		assert.Contains(t, value, `"logger":"logger_tests"`)
		assert.Contains(t, value, `"msg":"test error with no formatting"`)
		assert.Contains(t, value, `"error":"test error"`)
		assert.Contains(t, value, `"level":"error"`)
		assert.Contains(t, value, `"timestamp":"`)
	})

	t.Run("log errorf", func(t *testing.T) {
		buf.Reset()
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)

		Errorf(ctx, errors.New("test error"), "test %s", "info")
		value := buf.String()
		assert.Contains(t, value, `"logger":"logger_tests"`)
		assert.Contains(t, value, `"msg":"test info"`)
		assert.Contains(t, value, `"error":"test error"`)
		assert.Contains(t, value, `"level":"error"`)
		assert.Contains(t, value, `"timestamp":"`)
	})

	t.Run("log infof with http request", func(t *testing.T) {
		buf.Reset()
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
		assert.Contains(t, value, `"logger":"logger_tests"`)
		assert.Contains(t, value, `"msg":"test info"`)
		assert.Contains(t, value, `"req_url":"http://example.com/api/v1/health"`)
		assert.Contains(t, value, `"level":"info"`)
		assert.Contains(t, value, `"timestamp":"`)
		assert.Contains(t, value, `"req_params":{"`)
		assert.Contains(t, value, `"query_key":["query_value"]`)
		assert.Contains(t, value, `"req_headers":{"Accept":["application/json"]}`)
	})

	t.Run("log infof with http request containing authorization header", func(t *testing.T) {
		buf.Reset()
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)

		data := `{"testing-body":"test"}`
		req := httptest.NewRequest("GET", "http://example.com/api/v1/health", strings.NewReader(data))
		req.Header.Add("Accept", "application/json")
		req.Header.Add("Authorization", "Bearer "+"test-fake-bearer-token")

		q := req.URL.Query()
		q.Add("query_key", "query_value")
		q.Add("token", "query_token")
		req.URL.RawQuery = q.Encode()
		ctx.Request = req

		Infof(ctx, "test %s", "info")
		value := buf.String()
		assert.Contains(t, value, `"logger":"logger_tests"`)
		assert.Contains(t, value, `"msg":"test info"`)
		assert.Contains(t, value, `"req_url":"http://example.com/api/v1/health"`)
		assert.Contains(t, value, `"level":"info"`)
		assert.Contains(t, value, `"timestamp":"`)
		assert.Contains(t, value, `"req_params":{"`)
		assert.Contains(t, value, `"query_key":["query_value"]`)
		assert.Contains(t, value, `"token":["*****"]`)
		assert.Contains(t, value, `"req_headers":{"`)
		assert.Contains(t, value, `"Accept":["application/json"]`)
		assert.Contains(t, value, `"Authorization":"*****"`)
		assert.Contains(t, value, `"req_payload":"{\"testing-body\":\"test\"}"`)
	})

	t.Run("log infof withValues", func(t *testing.T) {
		buf.Reset()
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Set("subject", "test")

		WithValues(map[string]interface{}{"testing": "with-values"}).Infof(ctx, "test %s", "info")
		value := buf.String()
		assert.Contains(t, value, `"logger":"logger_tests"`)
		assert.Contains(t, value, `"msg":"test info"`)
		assert.Contains(t, value, `"testing":"with-values"`)
		assert.Contains(t, value, `"user_id":"test"`)
		assert.Contains(t, value, `"level":"info"`)
		assert.Contains(t, value, `"timestamp":"`)
	})

	t.Run("log infof with empty values", func(t *testing.T) {
		buf.Reset()
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)

		WithValues(map[string]interface{}{}).Infof(ctx, "test %s", "info")
		value := buf.String()
		assert.Contains(t, value, `"logger":"logger_tests"`)
		assert.Contains(t, value, `"msg":"test info"`)
		assert.Contains(t, value, `"level":"info"`)
	})

	t.Run("log infof with nil values", func(t *testing.T) {
		buf.Reset()
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)

		WithValues(nil).Infof(ctx, "test %s", "info")
		value := buf.String()
		assert.Contains(t, value, `"logger":"logger_tests"`)
		assert.Contains(t, value, `"msg":"test info"`)
		assert.Contains(t, value, `"level":"info"`)
	})

	t.Run("log infof setOutput when tags is set", func(t *testing.T) {
		buf.Reset()
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Set("subject", "test")

		WithValues(map[string]interface{}{"testing-2": "with-values-2"}).Infof(ctx, "test %s", "info")
		value := buf.String()
		assert.Contains(t, value, `"logger":"logger_tests"`)
		assert.Contains(t, value, `"msg":"test info"`)
		assert.Contains(t, value, `"testing-2":"with-values-2"`)
		assert.Contains(t, value, `"user_id":"test"`)
		assert.Contains(t, value, `"level":"info"`)
		assert.Contains(t, value, `"timestamp":"`)
	})
}
