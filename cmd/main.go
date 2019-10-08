package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/pkg/server"
)

var logr log.Logger

func main() {
	// create logrger and registry
	logr = *log.InitializeLogger("logger")

	// Parse flags
	var configFilePath string
	flag.StringVar(&configFilePath, "config", "", "path to the config file to read (if none is given, defaults will be used)")
	flag.Parse()

	// Override default -config switch with environment variable only if -config
	// switch was not explicitly given via the command line.
	configSwitchIsSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "config" {
			configSwitchIsSet = true
		}
	})
	if !configSwitchIsSet {
		if envConfigPath, ok := os.LookupEnv(configuration.EnvPrefix + "_CONFIG_FILE_PATH"); ok {
			configFilePath = envConfigPath
		}
	}

	srv, err := server.New(configFilePath)
	if err != nil {
		panic(err.Error())
	}

	err = srv.SetupRoutes()
	if err != nil {
		panic(err.Error())
	}

	routesToPrint := srv.GetRegisteredRoutes()
	logr.Infof(nil, "Configured routes: %s", routesToPrint)

	// listen concurrently to allow for graceful shutdown
	go func() {
		logr.Infof(nil, "Service Revision %s built on %s", configuration.Commit, configuration.BuildTime)
		logr.Infof(nil, "Listening on %q...", srv.Config().GetHTTPAddress())
		if err := srv.HTTPServer().ListenAndServe(); err != nil {
			logr.Info(nil, err.Error())
		}
	}()

	gracefulShutdown(srv.HTTPServer(), srv.Config().GetGracefulTimeout())
}

func gracefulShutdown(hs *http.Server, timeout time.Duration) {
	// For a channel used for notification of just one signal value, a buffer of
	// size 1 is sufficient.
	stop := make(chan os.Signal, 1)

	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C) or SIGTERM
	// (Ctrl+/). SIGKILL, SIGQUIT will not be caught.
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	sigReceived := <-stop
	logr.Infof(nil, "Signal received: %+v", sigReceived.String())

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	logr.Infof(nil, "\nShutdown with timeout: %s\n", timeout.String())
	if err := hs.Shutdown(ctx); err != nil {
		logr.Errorf(nil, err, "Shutdown error: %s\n", err.Error())
	} else {
		logr.Info(nil, "Server stopped.")
	}
}
