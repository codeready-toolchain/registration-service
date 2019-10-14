package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/server"
)

func main() {
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

	config, err := configuration.New(configFilePath)
	if err != nil {
		panic(err.Error())
	}

	srv, err := server.New(config)
	if err != nil {
		panic(err.Error())
	}

	err = srv.SetupRoutes()
	if err != nil {
		panic(err.Error())
	}

	routesToPrint := srv.GetRegisteredRoutes()
	srv.Logger().Printf("Configured routes: %s", routesToPrint)

	// listen concurrently to allow for graceful shutdown
	go func() {
		srv.Logger().Printf("Service Revision %s built on %s", configuration.Commit, configuration.BuildTime)
		srv.Logger().Printf("Listening on %q...", srv.Config().GetHTTPAddress())
		if err := srv.HTTPServer().ListenAndServe(); err != nil {
			srv.Logger().Println(err)
		}
	}()

	gracefulShutdown(srv.HTTPServer(), srv.Logger(), srv.Config().GetGracefulTimeout())
}

func gracefulShutdown(hs *http.Server, logger *log.Logger, timeout time.Duration) {
	// For a channel used for notification of just one signal value, a buffer of
	// size 1 is sufficient.
	stop := make(chan os.Signal, 1)

	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C) or SIGTERM
	// (Ctrl+/). SIGKILL, SIGQUIT will not be caught.
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	sigReceived := <-stop
	logger.Printf("Signal received: %+v", sigReceived)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	logger.Printf("\nShutdown with timeout: %s\n", timeout)
	if err := hs.Shutdown(ctx); err != nil {
		logger.Printf("Shutdown error: %v\n", err)
	} else {
		logger.Println("Server stopped.")
	}
}
