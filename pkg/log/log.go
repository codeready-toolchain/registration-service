package log

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/context"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	sync "github.com/matryer/resync"
	"github.com/operator-framework/operator-sdk/pkg/log/zap"
	"github.com/spf13/pflag"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
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
func Init(withName string, out io.Writer) {
	once.Do(func() {
		zapFlagSet := pflag.NewFlagSet("zap", pflag.ExitOnError)

		// Add the zap logger flag set to the CLI. The flag set must
		// be added before calling pflag.Parse().
		pflag.CommandLine.AddFlagSet(zapFlagSet)

		// Add flags registered by imported packages (e.g. glog and
		// controller-runtime)
		pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

		pflag.Parse()

		// Use a zap logr.Logger implementation. If none of the zap
		// flags are configured (or if the zap flag set is not being
		// used), this defaults to a production zap logger.
		//
		// The logger instantiated here can be changed to any logger
		// implementing the logr.Logger interface. This logger will
		// be propagated through the whole operator, generating
		// uniform and structured logs.
		if out == nil {
			logf.SetLogger(zap.Logger())
		} else {
			logf.SetLogger(zap.LoggerTo(out))
		}
		logger = newLogger(withName)
	})
}

func newLogger(withName string) *Logger {
	return &Logger{
		logr: logf.Log.WithName(withName),
		name: withName,
	}
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
	ctxInfo := addContextInfo(ctx)
	l.logr.Info(msg, ctxInfo...)
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
		subject := ctx.GetString(context.SubKey)
		if subject != "" {
			fields = append(fields, "user_id")
			fields = append(fields, subject)
		}
		username := ctx.GetString(context.UsernameKey)
		if username != "" {
			fields = append(fields, context.UsernameKey)
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
		fields = append(fields, fmt.Sprintf("%s://%s%s", url.Scheme, url.Host, url.Path))

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
