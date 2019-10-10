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
	"github.com/operator-framework/operator-sdk/pkg/log/zap"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	sync "github.com/matryer/resync"
	"github.com/spf13/pflag"
)

var (
	log  *Logger
	once sync.Once
)

type Logger struct {
	logr logr.Logger
}

// Init initializes the logger.
func Init(name string, out io.Writer, development bool) {
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
	logf.SetLogger(zap.Logger())

	once.Do(func() {
		log = newLogger(name, out, development)
	})
}

func newLogger(name string, out io.Writer, development bool) *Logger {
	return &Logger{
		logr: logf.ZapLoggerTo(out, development).WithName(name),
	}
}

// Info logs a non-error message.
func Info(ctx *gin.Context, msg string) {
	log.Info(ctx, msg)
}

// Infof logs a non-error formatted message.
func Infof(ctx *gin.Context, msg string, args ...string) {
	log.Infof(ctx, msg, args...)
}

// Error logs the error with the given message.
func Error(ctx *gin.Context, err error, msg string) {
	log.Error(ctx, err, msg)
}

// Errorf logs the error with the given formatted message.
func Errorf(ctx *gin.Context, err error, msg string, args ...string) {
	log.Errorf(ctx, err, msg, args...)
}

// WithValues creates a new logger with additional key-value pairs in the context
func WithValues(keysAndValues map[string]interface{}) *Logger {
	return log.WithValues(keysAndValues)
}

// Info logs are used for non-error messages. It will log a message with
// the given key/value pairs as context.
func (l *Logger) Info(ctx *gin.Context, msg string) {
	log.Infof(ctx, msg)
}

// Infof logs are used for non-error messages. It will log a message with
// the given key/value pairs as context.
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

// Error logs are used for logging errors. It will log the error with the given
// message and key/value pairs as context.
func (l *Logger) Error(ctx *gin.Context, err error, msg string) {
	l.Errorf(ctx, err, msg)
}

// Errorf logs are used for logging errors. It will log the error with the given
// message and key/value pairs as context.
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

// WithValues appends tags to the logger.
func (l *Logger) WithValues(keysAndValues map[string]interface{}) *Logger {
	if len(keysAndValues) > 0 {
		// ZapLoggerTo, name and WithValues all return new logger instances.
		// The logger must be set again with the values stored in the Logger struct.
		log.logr = log.logr.WithValues(slice(keysAndValues)...)
	}
	return log
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
