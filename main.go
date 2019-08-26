//go:generate go run -tags=dev pkg/static/assets_generate.go

package main

import (
	"context"
	"flag"
	"log"
	//	"crypto/tls"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/registrationserver"
)

// Version is the service version
const Version string = "0.0.1"

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

	srv, err := registrationserver.New(configFilePath)
	if err != nil {
		panic(err.Error())
	}

	// setting the version of the service from the const value
	srv.Config().GetViperInstance().Set(configuration.VersionKey, Version)

	err = srv.SetupRoutes()
	if err != nil {
		panic(err.Error())
	}

	routesToPrint, err := srv.GetRegisteredRoutes()
	if err != nil {
		panic(err.Error())
	}
	srv.Logger().Print(routesToPrint)

	// listen concurrently to allow for graceful shutdown
	go func() {
		if srv.Config().IsHTTPInsecure() {
			srv.Logger().Println("WARNING: running in insecure mode, http only.")
			srv.Logger().Printf("Listening on %q...", srv.Config().GetHTTPAddress())
			if err := srv.HTTPServer().ListenAndServe(); err != nil {
				srv.Logger().Println(err)
			}
		} else {
			srv.Logger().Println("running in secure mode, https only.")
			srv.Logger().Printf("Listening on %q...", srv.Config().GetHTTPAddress())
			if err := srv.HTTPServer().ListenAndServeTLS(srv.Config().GetHTTPCertPath(), srv.Config().GetHTTPKeyPath()); err != nil {
				srv.Logger().Println(err)
			}
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
