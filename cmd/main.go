package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/metrics"
	"github.com/codeready-toolchain/toolchain-common/pkg/cluster"
	"github.com/prometheus/client_golang/prometheus"
	controllerlog "sigs.k8s.io/controller-runtime/pkg/log"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/auth"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/informers"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/pkg/proxy"
	"github.com/codeready-toolchain/registration-service/pkg/server"
	commonconfig "github.com/codeready-toolchain/toolchain-common/pkg/configuration"

	errs "github.com/pkg/errors"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	// create logger and registry
	log.Init("registration-service",
		zap.UseDevMode(true),
		zap.Encoder(zapcore.NewJSONEncoder(zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		})),
	)

	_, found := os.LookupEnv(commonconfig.WatchNamespaceEnvVar)
	if !found {
		panic(fmt.Errorf("%s not set", commonconfig.WatchNamespaceEnvVar))
	}

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		os.Exit(1)
	}

	// create runtime client
	cl, err := configClient(cfg)
	if err != nil {
		panic(err.Error())
	}

	crtConfig, err := configuration.ForceLoadRegistrationServiceConfig(cl)
	if err != nil {
		panic(fmt.Sprintf("failed to initialize configuration: %s", err.Error()))
	}
	crtConfig.Print()

	if crtConfig.Verification().CaptchaEnabled() {
		if err := createCaptchaFileFromSecret(crtConfig); err != nil {
			panic(fmt.Sprintf("failed to create captcha file: %s", err.Error()))
		}

		// set application credentials env var required for recaptcha client
		if err := os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", configuration.CaptchaFilePath); err != nil {
			panic(fmt.Sprintf("cannot set captcha credentials: %s", err.Error()))
		}
	}

	informer, informerShutdown, err := informers.StartInformer(cfg)
	if err != nil {
		panic(err.Error())
	}

	app, err := server.NewInClusterApplication(*informer)
	if err != nil {
		panic(err.Error())
	}
	// Initialize toolchain cluster cache service
	// let's cache the member clusters before we start the services,
	// this will speed up the first request
	cacheLog := controllerlog.Log.WithName("registration-service")
	cluster.NewToolchainClusterService(cl, cacheLog, configuration.Namespace(), 5*time.Second)
	cluster.GetMemberClusters()

	_, err = auth.InitializeDefaultTokenParser()
	if err != nil {
		panic(errs.Wrap(err, "failed to init default token parser"))
	}

	// Register metrics
	proxyMetrics := metrics.NewProxyMetrics(prometheus.NewRegistry())
	// Start metrics server
	metricsSrv := proxyMetrics.StartMetricsServer()

	// Start the proxy server
	p, err := proxy.NewProxy(app, proxyMetrics, cluster.GetMemberClusters)
	if err != nil {
		panic(errs.Wrap(err, "failed to create proxy"))
	}
	proxySrv := p.StartProxy(proxy.DefaultPort)

	// stop the informer when proxy server shuts down
	proxySrv.RegisterOnShutdown(func() {
		informerShutdown <- struct{}{}
	})

	srv := server.New(app)

	err = srv.SetupRoutes(proxy.DefaultPort)
	if err != nil {
		panic(err.Error())
	}

	routesToPrint := srv.GetRegisteredRoutes()
	log.Infof(nil, "Configured routes: %s", routesToPrint)

	// listen concurrently to allow for graceful shutdown
	go func() {
		log.Infof(nil, "Service Revision %s built on %s", configuration.Commit, configuration.BuildTime)
		log.Infof(nil, "Listening on %q...", configuration.HTTPAddress)
		if err := srv.HTTPServer().ListenAndServe(); err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				log.Info(nil, fmt.Sprintf("%s - this is expected when server shutdown has been initiated", err.Error()))
			} else {
				log.Error(nil, err, err.Error())
			}
		}
	}()

	// update cache every 10 seconds
	go func() {
		for {
			if _, err := configuration.ForceLoadRegistrationServiceConfig(cl); err != nil {
				log.Error(nil, err, "failed to update the configuration cache")
			}
			time.Sleep(10 * time.Second)
		}
	}()

	gracefulShutdown(configuration.GracefulTimeout, srv.HTTPServer(), proxySrv, metricsSrv)
}

func gracefulShutdown(timeout time.Duration, hs ...*http.Server) {
	// For a channel used for notification of just one signal value, a buffer of
	// size 1 is sufficient.
	stop := make(chan os.Signal, 1)

	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C) or SIGTERM
	// (Ctrl+/). SIGKILL, SIGQUIT will not be caught.
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	sigReceived := <-stop
	log.Infof(nil, "Signal received: %+v", sigReceived.String())

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	log.Infof(nil, "Shutdown with timeout: %s", timeout.String())
	for _, s := range hs {
		if err := s.Shutdown(ctx); err != nil {
			log.Errorf(nil, err, "Shutdown error")
		} else {
			log.Info(nil, "Server stopped.")
		}
	}
}

func configClient(cfg *rest.Config) (client.Client, error) {
	scheme := runtime.NewScheme()
	var AddToSchemes runtime.SchemeBuilder
	addToSchemes := append(AddToSchemes,
		corev1.AddToScheme,
		toolchainv1alpha1.AddToScheme)
	err := addToSchemes.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}
	return client.New(cfg, client.Options{
		Scheme: scheme,
	})
}

func createCaptchaFileFromSecret(cfg configuration.RegistrationServiceConfig) error {
	contents := cfg.Verification().CaptchaServiceAccountFileContents()
	if err := os.WriteFile(configuration.CaptchaFilePath, []byte(contents), 0600); err != nil {
		return errs.Wrap(err, "error writing captcha file")
	}
	return nil
}
