package log

import (
	"flag"
	"fmt"
	"io"
	"os"

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
	Errorf(ctx *gin.Context, err error, msg string, args ...string)
	Infof(ctx *gin.Context, msg string, args ...string)
	WithValues(keysAndValues ...interface{})
	SetOutput(out io.Writer, isTestingMode bool)
}


type Logger struct {
	lgr           logr.Logger
	name          string
	tags          []interface{}
	out           io.Writer
	isTestingMode bool
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

	// set the logger.
	log = Logger{
		name:          withName,
		lgr:           logf.Log.WithName(withName),
		out:           os.Stdout,
		isTestingMode: false,
	}

	return &log
}

func (p *Logger) SetOutput(out io.Writer, isTestingMode bool) *Logger {
	// WithValues, WithName and ZapLoggerTo all result in a new logger instance.
	// The values stored in the Logger struct must be set again.
	if len(log.tags) > 0 {
		log.lgr = logf.ZapLoggerTo(out, isTestingMode).WithName(log.name).WithValues(log.tags...)
	} else {
		log.lgr = logf.ZapLoggerTo(out, isTestingMode).WithName(log.name)
	}

	log.out = out
	log.isTestingMode = isTestingMode
	return &log
}

// GetLogger returns the current logger object.
func GetLogger() *Logger {
	return &log
}

// Info logs are used for non-error messages. It will log a message with
// the given key/value pairs as context.
func (p *Logger) Info(ctx *gin.Context, msg string) *Logger {
	return p.Infof(ctx, msg, "")
}

// Infof logs are used for non-error messages. It will log a message with
// the given key/value pairs as context.
func (p *Logger) Infof(ctx *gin.Context, msg string, args ...string) *Logger {
	ctxInfo := addContextInfo(ctx)
	arguments := make([]interface{}, len(args))
	for i, arg := range args {
		arguments[i] = arg
	}
	p.lgr.Info(fmt.Sprintf(msg, arguments...), ctxInfo...)
	return p
}

// Error logs are used for logging errors. It will log the error with the given
// message and key/value pairs as context.
func (p *Logger) Error(ctx *gin.Context, err error, msg string) *Logger {
	return p.Errorf(ctx, err, msg, "")
}

// Errorf logs are used for logging errors. It will log the error with the given
// message and key/value pairs as context.
func (p *Logger) Errorf(ctx *gin.Context, err error, msg string, args ...string) *Logger {
	ctxInfo := addContextInfo(ctx)
	arguments := make([]interface{}, len(args))
	for i, arg := range args {
		arguments[i] = arg
	}
	p.lgr.Error(err, fmt.Sprintf(msg, arguments...), ctxInfo...)
	return p
}

// WithValues appends tags to the logger.
func (p *Logger) WithValues(keysAndValues ...interface{}) *Logger {
	if len(keysAndValues) > 0 {
		tags := append([]interface{}(nil), p.tags...)
		tags = append(tags, keysAndValues...)
		p.tags = tags
		// ZapLoggerTo, WithName and WithValues all return new logger instances.
		// The logger must be set again with the values stored in the Logger struct.
		p.lgr = logf.ZapLoggerTo(p.out, p.isTestingMode).WithName(p.name).WithValues(tags...)
	}
	return p
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
				v = append(v, url.Scheme+"://"+url.Host+url.Path)
			}
		}
	}

	return v
}
