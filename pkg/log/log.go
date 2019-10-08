package log

import (
	"flag"
	"fmt"
	"io"

	"github.com/operator-framework/operator-sdk/pkg/log/zap"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
)

var (
	log Logger
)

// Log interface for the logger.
type Log interface {
	Errorf(ctx *gin.Context, err error, msg string, args ...interface{})
	Infof(ctx *gin.Context, msg string, args ...interface{})
	WithValues(keysAndValues ...interface{})
	SetOutput(out io.Writer, isTestingMode bool)

}

type Logger struct {
	lgr logr.Logger
	name string
	tags  []interface{}
}

// InitializeLogger initializes the logger.
func InitializeLogger(withName string) *Logger {

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

	log = Logger{
		name: withName, 
		lgr: logf.Log.WithName(withName), 
	}

	return &log
}

func (p *Logger)SetOutput(out io.Writer, isTestingMode bool) *Logger {
	log.lgr = logf.ZapLoggerTo(out, isTestingMode).WithName(log.name)
	if len(log.tags) > 0 {
		log.lgr.WithValues(log.tags...)
	}
	
	return &log
}

// GetLogger returns the current logger object.
func GetLogger() *Logger {
	return &log
}

// Info logs are used for non-error messages. It will log a message with
// the given key/value pairs as context.
func (p *Logger)Infof(ctx *gin.Context, msg string, args ...interface{}) *Logger {
	ctxInfo := addContextInfo(ctx)
	log.lgr.Info(fmt.Sprintf(msg, args...), ctxInfo...)
	return &log
}

// Error logs are used for logging errors. It will log the error with the given
// message and key/value pairs as context.
func (p *Logger)Errorf(ctx *gin.Context, err error, msg string, args ...interface{}) *Logger {
	ctxInfo := addContextInfo(ctx)
	log.lgr.Error(err, fmt.Sprintf(msg, args...), ctxInfo...)
	return &log
}

// WithValues appends tags to the logger.
func (p *Logger)WithValues(keysAndValues ...interface{}) *Logger {
	if len(keysAndValues) > 0 {
		tags := append([]interface{}(nil), log.tags...)
		tags = append(tags, keysAndValues...)
		log = Logger{
			name: log.name, 
			lgr: logf.Log.WithName(log.name), 
			tags: tags,
		}
		log.lgr.WithValues(tags...)
	}
	return &log
}

// addContextInfo adds fields extracted from the context to the info/error
// log messages.
func addContextInfo(ctx *gin.Context) []interface{} {
	var v []interface{}

	if ctx != nil {
		subject := ctx.GetString("subject")
		if subject != "" {
			v = append(v, "user_id")
			v = append(v, subject)
		}

		if ctx.Request != nil {
			url := ctx.Request.URL
			if url != nil {
				v = append(v, "req_url")
				v = append(v, url.Scheme + "://" + url.Host + url.Path)
			}
		}
	}

	return v
}
