package log

import (
	"flag"
	"fmt"
	"io"

	"github.com/operator-framework/operator-sdk/pkg/log/zap"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/spf13/pflag"
	"github.com/go-logr/logr"
	"github.com/gin-gonic/gin"
)

var (
	log logr.Logger
)

// InitializeLogger initializes the logger.
func InitializeLogger(withName string) {
	log = logf.Log.WithName(withName)

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
}

func SetOutput(out io.Writer, isTestingMode bool, withName string) {
	log = logf.ZapLoggerTo(out, isTestingMode).WithName(withName)
}

// Logger returns the current logger object.
func Logger() logr.Logger {
	return log
}

// Info logs are used for non-error messages. It will log a message with 
// the given key/value pairs as context.
func Info(ctx *gin.Context, msg string, v ...interface{}) {
	log.Info(fmt.Sprintf(msg, addContextInfo(ctx, v...)))
}

// Error logs are used for logging errors. It will log the error with the given 
// message and key/value pairs as context.
func Error(ctx *gin.Context, err error, msg string, v ...interface{}) {
	junk := addContextInfo(ctx, v...)
	log.Error(err, msg, junk...)
}

// addContextInfo adds fields extracted from the context to the info/error
// log messages.
func addContextInfo(ctx *gin.Context, v ...interface{}) []interface{} {
	if ctx != nil {
		subject := ctx.GetString("subject")
		if subject != ""{
			v = append(v, fmt.Sprintf("context subject"))
			v = append(v, fmt.Sprintf(subject))
		}

		if ctx.Request != nil {
			url := ctx.Request.URL
			if url != nil {
				v = append(v, fmt.Sprintf("context host: %s", url.Host))
			}
		}
	}

	return v
}
