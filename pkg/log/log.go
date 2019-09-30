package log

import (
	"io"
	"log"
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
func Fatal(v ...interface{}) {
	logger.Fatal(v...)
}

// Fatalf is equivalent to l.Printf() followed by a call to os.Exit(1).
func Fatalf(format string, v ...interface{}) {
	logger.Fatalf(format, v...)
}

// Fatalln is equivalent to l.Println() followed by a call to os.Exit(1).
func Fatalln(v ...interface{}) {
	logger.Fatalln(v...)
}

// Flags returns the output flags for the logger.
func Flags() int {
	return logger.Flags()
}

// Panic is equivalent to l.Print() followed by a call to panic().
func Panic(v ...interface{}) {
	logger.Panic(v...)
}

// Panicf is equivalent to l.Printf() followed by a call to panic().
func Panicf(format string, v ...interface{}) {
	logger.Panicf(format, v...)
}

// Panicln is equivalent to l.Println() followed by a call to panic().
func Panicln(v ...interface{}) {
	logger.Panicln(v...)
}

// Prefix returns the output prefix for the logger.
func Prefix() string {
	return logger.Prefix()
}

// Print calls l.Output to print to the logger. Arguments are handled in the manner of fmt.Print.
func Print(v ...interface{}) {
	logger.Print(v...)
}

// Printf calls l.Output to print to the logger. Arguments are handled in the manner of fmt.Printf.
func Printf(format string, v ...interface{}) {
	logger.Printf(format, v...)
}

// Println calls l.Output to print to the logger. Arguments are handled in the manner of fmt.Println.
func Println(v ...interface{}) {
	logger.Println(v...)
}
