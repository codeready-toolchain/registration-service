package log

import (
	"fmt"
	"io"
	"log"

	"github.com/gin-gonic/gin"
)

var (
	logger = &log.Logger{}
)

// InitializeLogger initializes the logger
func InitializeLogger(out io.Writer, prefix string, flag int) {
	logger = log.New(out, prefix, flag)
}

// Logger returns the current logger object.
func Logger() *log.Logger {
	return logger
}

// Fatal is equivalent to l.Print() followed by a call to os.Exit(1).
func Fatal(ctx *gin.Context, v ...interface{}) {
	logger.Fatal(addContextKeys(ctx, v))
}

// Fatalf is equivalent to l.Printf() followed by a call to os.Exit(1).
func Fatalf(ctx *gin.Context, format string, v ...interface{}) {
	logger.Fatalf(format, v...)
}

// Fatalln is equivalent to l.Println() followed by a call to os.Exit(1).
func Fatalln(ctx *gin.Context, v ...interface{}) {
	logger.Fatalln(addContextKeys(ctx, v))
}

// Flags returns the output flags for the logger.
func Flags() int {
	return logger.Flags()
}

// Panic is equivalent to l.Print() followed by a call to panic().
func Panic(ctx *gin.Context, v ...interface{}) {
	logger.Panic(addContextKeys(ctx, v))
}

// Panicf is equivalent to l.Printf() followed by a call to panic().
func Panicf(ctx *gin.Context, format string, v ...interface{}) {
	logger.Panicf(format, v...)
}

// Panicln is equivalent to l.Println() followed by a call to panic().
func Panicln(ctx *gin.Context, v ...interface{}) {
	logger.Panicln(addContextKeys(ctx, v))
}

// Prefix returns the output prefix for the logger.
func Prefix() string {
	return logger.Prefix()
}

// Print calls l.Output to print to the logger. Arguments are handled in the manner of fmt.Print.
func Print(ctx *gin.Context, v ...interface{}) {
	logger.Print(addContextKeys(ctx, v))
}

// Printf calls l.Output to print to the logger. Arguments are handled in the manner of fmt.Printf.
func Printf(ctx *gin.Context, format string, v ...interface{}) {
	logger.Printf(format, v...)
}

// Println calls l.Output to print to the logger. Arguments are handled in the manner of fmt.Println.
func Println(ctx *gin.Context, v ...interface{}) {
	logger.Println(addContextKeys(ctx, v))
}

func addContextKeys(ctx *gin.Context, v ...interface{}) []interface{} {
	if ctx != nil {

		subject, e := ctx.Get("subject")
		if e {
			v = append(v, fmt.Sprintf("context subject: %s", subject))
		}
		subscription, e := ctx.Get("subscription")
		if e {
			v = append(v, fmt.Sprintf("context subscription: %s", subscription))
		}
		url, e := ctx.Get("url")
		if e {
			v = append(v, fmt.Sprintf("context url: %s", url))
		}
	}

	return v
}
