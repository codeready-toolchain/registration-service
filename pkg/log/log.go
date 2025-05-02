package log

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/labstack/echo/v4"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	sync "github.com/matryer/resync"
	"github.com/spf13/pflag"
	klogv1 "k8s.io/klog"
	klogv2 "k8s.io/klog/v2"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
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
func Init(withName string, opts ...zap.Opts) {
	once.Do(func() {
		zapFlagSet := pflag.NewFlagSet("zap", pflag.ExitOnError)

		// Add the zap logger flag set to the CLI. The flag set must
		// be added before calling pflag.Parse().
		pflag.CommandLine.AddFlagSet(zapFlagSet)

		// Add flags registered by imported packages (e.g. glog and
		// controller-runtime)
		pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

		// Use a zap logr.Logger implementation. If none of the zap
		// flags are configured (or if the zap flag set is not being
		// used), this defaults to a production zap logger.
		//
		// The logger instantiated here can be changed to any logger
		// implementing the logr.Logger interface. This logger will
		// be propagated through the whole operator, generating
		// uniform and structured logs.
		logf.SetLogger(zap.New(opts...))
		logger = newLogger(withName)

		// also set the client-go logger so we get the same JSON output
		klogv2.SetLogger(zap.New(opts...))

		// see https://github.com/kubernetes/klog#coexisting-with-klogv2
		// BEGIN : hack to redirect klogv1 calls to klog v2
		// Tell klog NOT to log into STDERR. Otherwise, we risk
		// certain kinds of API errors getting logged into a directory not
		// available in a `FROM scratch` Docker container, causing us to abort
		var klogv1Flags flag.FlagSet
		klogv1.InitFlags(&klogv1Flags)
		setupLog := logf.Log.WithName("setup")
		if err := klogv1Flags.Set("logtostderr", "false"); err != nil { // By default klog v1 logs to stderr, switch that off
			setupLog.Error(err, "")
			os.Exit(1)
		}
		if err := klogv1Flags.Set("stderrthreshold", "FATAL"); err != nil { // stderrthreshold defaults to ERROR, so we don't get anything in stderr
			setupLog.Error(err, "")
			os.Exit(1)
		}
		klogv1.SetOutputBySeverity("INFO", klogWriter{}) // tell klog v1 to use the custom writer
		// END : hack to redirect klogv1 calls to klog v2

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

// InfoEchof logs a non-error formatted message for echo events.
func InfoEchof(ctx echo.Context, msg string, args ...string) {
	logger.InfoEchof(ctx, msg, args...)
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
	l.infof(ctxInfo, msg, args...)
}

// InfoEchof logs a non-error formatted message for echo events.
func (l *Logger) InfoEchof(ctx echo.Context, msg string, args ...string) {
	userID, _ := ctx.Get(context.SubKey).(string)
	username, _ := ctx.Get(context.UsernameKey).(string)
	ctxFields := genericContext(userID, username)

	workspace, _ := ctx.Get(context.WorkspaceKey).(string)
	ctxFields = append(ctxFields, "workspace")
	ctxFields = append(ctxFields, workspace)

	ctxFields = append(ctxFields, "method")
	ctxFields = append(ctxFields, ctx.Request().Method)

	ctxFields = append(ctxFields, "url")
	ctxFields = append(ctxFields, ctx.Request().URL)

	if impersonateUser, ok := ctx.Get(context.ImpersonateUser).(string); ok {
		ctxFields = append(ctxFields, "impersonate-user", impersonateUser)
	}

	if publicViewerEnabled, ok := ctx.Get(context.PublicViewerEnabled).(bool); ok {
		ctxFields = append(ctxFields, "public-viewer-enabled", publicViewerEnabled)
	}

	l.infof(ctxFields, msg, args...)
}

func (l *Logger) infof(ctx []interface{}, msg string, args ...string) {
	arguments := make([]interface{}, len(args))
	for i, arg := range args {
		arguments[i] = arg
	}
	if len(arguments) > 0 {
		l.logr.Info(fmt.Sprintf(msg, arguments...), ctx...)
	} else {
		l.logr.Info(msg, ctx...)
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
	if ctx != nil {
		subject := ctx.GetString(context.SubKey)
		username := ctx.GetString(context.UsernameKey)
		fields := genericContext(subject, username)
		if ctx.Request != nil {
			fields = append(fields, addRequestInfo(ctx.Request)...)
		}
		return fields
	}

	return genericContext("", "")
}

func genericContext(subject, username string) []interface{} {
	var fields []interface{}

	// TODO: we probably don't need the timestamp as it is automatically added to the log by the logger
	currentTime := time.Now()
	fields = append(fields, "timestamp")
	fields = append(fields, currentTime.Format(time.RFC1123Z))
	// TODO: we can drop the commit as well - printing out the commit for every single line is a kind of overkill
	fields = append(fields, "commit")
	if len(configuration.Commit) > 7 {
		fields = append(fields, configuration.Commit[0:7])
	} else {
		fields = append(fields, configuration.Commit)
	}

	if subject != "" {
		fields = append(fields, "user_id")
		fields = append(fields, subject)
	}
	if username != "" {
		fields = append(fields, context.UsernameKey)
		fields = append(fields, username)
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
		// Once req.Body is read, it is empty. Restore its contents by
		// assigning a new reader with the same contents.
		req.Body = io.NopCloser(bytes.NewReader(buf.Bytes()))
	}

	return fields
}

// OutputCallDepth is the stack depth where we can find the origin of this call
const OutputCallDepth = 6

// DefaultPrefixLength is the length of the log prefix that we have to strip out
const DefaultPrefixLength = 53

// klogWriter is used in SetOutputBySeverity call below to redirect
// any calls to klogv1 to end up in klogv2
type klogWriter struct{}

func (kw klogWriter) Write(p []byte) (n int, err error) {
	if len(p) < DefaultPrefixLength {
		klogv2.InfoDepth(OutputCallDepth, string(p))
		return len(p), nil
	}
	switch p[0] {
	case 'I':
		klogv2.InfoDepth(OutputCallDepth, string(p[DefaultPrefixLength:]))
	case 'W':
		klogv2.WarningDepth(OutputCallDepth, string(p[DefaultPrefixLength:]))
	case 'E':
		klogv2.ErrorDepth(OutputCallDepth, string(p[DefaultPrefixLength:]))
	case 'F':
		klogv2.FatalDepth(OutputCallDepth, string(p[DefaultPrefixLength:]))
	default:
		klogv2.InfoDepth(OutputCallDepth, string(p[DefaultPrefixLength:]))
	}
	return len(p), nil
}
