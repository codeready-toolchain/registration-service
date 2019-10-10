package log

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/middleware"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
)

var (
	logger *Logger
	once   sync.Once
)

// Logger implements log.Logger
type Logger struct {
	logr logr.Logger
	name string
}

// Init initializes the logger.
func Init(withName string) {
	once.Do(func() {
		logger = newLogger(withName)
	})
}

func newLogger(withName string) *Logger {
	return &Logger{
		logr: logf.Log.WithName(withName),
		name: withName,
	}
}

// ZapLoggerTo returns a new Logger implementation using Zap which logs
// to the given destination, instead of stderr.
func ZapLoggerTo(out io.Writer, development bool) *Logger {
	nl := newLogger(logger.name)
	nl.logr = logf.ZapLoggerTo(out, development).WithName(logger.name)
	return nl
}

// Info logs a non-error message.
func Info(ctx *gin.Context, msg string) {
	logger.Info(ctx, msg)
}

// Infof logs a non-error formatted message.
func Infof(ctx *gin.Context, msg string, args ...string) {
	logger.Infof(ctx, msg, args...)
}

// Error logs the error with the given message.
func Error(ctx *gin.Context, err error, msg string) {
	logger.Error(ctx, err, msg)
}

// Errorf logs the error with the given formatted message.
func Errorf(ctx *gin.Context, err error, msg string, args ...string) {
	logger.Errorf(ctx, err, msg, args...)
}

// WithValues creates a new logger with additional key-value pairs in the context
func WithValues(keysAndValues map[string]interface{}) *Logger {
	return logger.WithValues(keysAndValues)
}

// Info logs a non-error message.
func (l *Logger) Info(ctx *gin.Context, msg string) {
	Infof(ctx, msg)
}

// Infof logs a non-error formatted message.
func (l *Logger) Infof(ctx *gin.Context, msg string, args ...string) {
	ctxInfo := addContextInfo(ctx)
	arguments := make([]interface{}, len(args))
	for i, arg := range args {
		arguments[i] = arg
	}
	if len(arguments) > 0 {
		l.logr.Info(fmt.Sprintf(msg, arguments...), ctxInfo...)
	} else {
		l.logr.Info(msg, ctxInfo...)
	}
}

// Error logs the error with the given message.
func (l *Logger) Error(ctx *gin.Context, err error, msg string) {
	l.Errorf(ctx, err, msg)
}

// Errorf logs the error with the given formatted message.
func (l *Logger) Errorf(ctx *gin.Context, err error, msg string, args ...string) {
	ctxInfo := addContextInfo(ctx)
	arguments := make([]interface{}, len(args))
	for i, arg := range args {
		arguments[i] = arg
	}

	if len(arguments) > 0 {
		l.logr.Error(err, fmt.Sprintf(msg, arguments...), ctxInfo...)
	} else {
		l.logr.Error(err, msg, ctxInfo...)
	}
}

// WithValues creates a new logger with additional key-value pairs in the context
func (l *Logger) WithValues(keysAndValues map[string]interface{}) *Logger {
	if len(keysAndValues) > 0 {
		nl := newLogger(logger.name)
		nl.logr = nl.logr.WithValues(slice(keysAndValues)...)
		return nl
	}
	return l
}

func slice(keysAndValues map[string]interface{}) []interface{} {
	tags := make([]interface{}, 0, len(keysAndValues)*2)
	for k, v := range keysAndValues {
		tags = append(tags, k)
		tags = append(tags, v)
	}
	return tags
}

// addContextInfo adds fields extracted from the context to the info/error
// log messages.
func addContextInfo(ctx *gin.Context) []interface{} {
	var fields []interface{}

	currentTime := time.Now()
	fields = append(fields, "timestamp")
	fields = append(fields, currentTime.Format(time.RFC1123Z))

	if ctx != nil {
		subject := ctx.GetString(middleware.SubKey)
		if subject != "" {
			fields = append(fields, "user_id")
			fields = append(fields, subject)
		}
		username := ctx.GetString(middleware.UsernameKey)
		if subject != "" {
			fields = append(fields, "username")
			fields = append(fields, username)
		}

		if ctx.Request != nil {
			fields = append(fields, addRequestInfo(ctx.Request)...)
		}
	}

	return fields
}

// addRequestInfo adds fields extracted from context.Request.
func addRequestInfo(req *http.Request) []interface{} {
	var fields []interface{}
	url := req.URL

	if url != nil {
		fields = append(fields, "req_url")
		fields = append(fields, url.Scheme+"://"+url.Host+url.Path)

		reqParams := req.URL.Query()
		if len(reqParams) > 0 {
			params := make(map[string][]string)
			for name, values := range reqParams {
				if strings.ToLower(name) != "token" {
					params[name] = values
				} else {
					// Hide sensitive information
					params[name] = []string{"*****"}
				}
			}
			fields = append(fields, "req_params")
			fields = append(fields, params)
		}
	}

	if len(req.Header) > 0 {
		headers := make(map[string]interface{}, len(req.Header))
		for k, v := range req.Header {
			// Hide sensitive information
			if k == "Authorization" || k == "Cookie" {
				headers[k] = "*****"
			} else {
				headers[k] = v
			}
		}
		fields = append(fields, "req_headers")
		fields = append(fields, headers)
	}
	if req.ContentLength > 0 {
		buf := new(bytes.Buffer)
		_, err := buf.ReadFrom(req.Body)
		var newStr string
		if err != nil {
			newStr = "<invalid data>"
		} else {
			newStr = buf.String()
		}
		fields = append(fields, "req_payload")
		fields = append(fields, newStr)
	}

	return fields
}
