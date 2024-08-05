package log

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/gin-gonic/gin"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		assert.Contains(t, value, `"user_id":"test"`) // subject -> user_id
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
		assert.Contains(t, value, `"user_id":"test"`) // subject -> user_id
		assert.Contains(t, value, `"username":"usernametest"`)
		assert.Contains(t, value, `"level":"info"`)
		assert.Contains(t, value, `"timestamp":"`)
	})

	t.Run("log infoEchof", func(t *testing.T) {
		tt := map[string]struct {
			name        string
			contains    string
			notContains string
			ctxSet      map[string]interface{}
		}{
			"default": {},
			"impersonate-user is set": {
				ctxSet:   map[string]interface{}{context.ImpersonateUser: "user"},
				contains: `"impersonate-user":"user"`,
			},
			"impersonate-user is not set": {
				ctxSet:      map[string]interface{}{},
				notContains: `impersonate-user`,
			},
			"public-viewer-enabled is set to true": {
				ctxSet:   map[string]interface{}{context.PublicViewerEnabled: true},
				contains: `"public-viewer-enabled":true`,
			},
			"public-viewer-enabled is set to false": {
				ctxSet:   map[string]interface{}{context.PublicViewerEnabled: false},
				contains: `"public-viewer-enabled":false`,
			},
			"public-viewer-enabled is not set": {
				ctxSet:      map[string]interface{}{},
				notContains: `public-viewer-enabled`,
			},
		}

		for _, tc := range tt {
			t.Run(tc.name, func(t *testing.T) {
				buf.Reset()
				req := httptest.NewRequest(http.MethodGet, "https://api-server.com/api/workspaces/path", strings.NewReader("{}"))
				rec := httptest.NewRecorder()
				ctx := echo.New().NewContext(req, rec)
				ctx.Set(context.SubKey, "test")
				ctx.Set(context.UsernameKey, "usernametest")
				ctx.Set(context.WorkspaceKey, "coolworkspace")
				for k, v := range tc.ctxSet {
					ctx.Set(k, v)
				}

				InfoEchof(ctx, "test %s", "info")
				value := buf.String()
				assert.Contains(t, value, `"logger":"logger_tests"`)
				assert.Contains(t, value, `"msg":"test info"`)
				assert.Contains(t, value, `"user_id":"test"`) // subject -> user_id
				assert.Contains(t, value, `"username":"usernametest"`)
				assert.Contains(t, value, `"level":"info"`)
				assert.Contains(t, value, `"timestamp":"`)
				assert.Contains(t, value, `"workspace":"coolworkspace"`)
				assert.Contains(t, value, `"method":"GET"`)
				assert.Contains(t, value, `"url":"https://api-server.com/api/workspaces/path"`)

				if tc.contains != "" {
					assert.Contains(t, value, tc.contains)
				}
				if tc.notContains != "" {
					assert.NotContains(t, value, tc.notContains)
				}
			})
		}
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
		buf := new(bytes.Buffer)
		_, err := buf.ReadFrom(req.Body)
		require.NoError(t, err, "it should still be possible to read the body after it was passed to the logs")
		assert.Equal(t, `{"testing-body":"test"}`, buf.String(), "body contents should be unchanged")
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
