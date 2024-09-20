package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/auth"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/log"
	"github.com/codeready-toolchain/registration-service/pkg/proxy"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/metrics"
	"github.com/codeready-toolchain/registration-service/pkg/server"
	"github.com/codeready-toolchain/toolchain-common/pkg/cluster"
	commonconfig "github.com/codeready-toolchain/toolchain-common/pkg/configuration"
	errs "github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	runtimecluster "sigs.k8s.io/controller-runtime/pkg/cluster"
	controllerlog "sigs.k8s.io/controller-runtime/pkg/log"
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

	ctx := controllerruntime.SetupSignalHandler()

	// create cached runtime client
	cl, err := newCachedClient(ctx, cfg)
	if err != nil {
		panic(err.Error())
	}

	configuration.SetClient(cl)
	crtConfig := configuration.GetRegistrationServiceConfig()
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

	app, err := server.NewInClusterApplication(cl)
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

	// ---------------------------------------------
	// API Proxy
	// ---------------------------------------------

	// API Proxy metrics server
	proxyRegistry := prometheus.NewRegistry()
	proxyMetrics := metrics.NewProxyMetrics(proxyRegistry)
	proxyMetricsSrv := proxy.StartMetricsServer(proxyRegistry, proxy.ProxyMetricsPort)
	// Proxy API server
	p, err := proxy.NewProxy(app, proxyMetrics, cluster.GetMemberClusters)
	if err != nil {
		panic(errs.Wrap(err, "failed to create proxy"))
	}
	proxySrv := p.StartProxy(proxy.DefaultPort)

	// ---------------------------------------------
	// Registration Service
	// ---------------------------------------------
	regsvcRegistry := prometheus.NewRegistry()
	regsvcMetricsSrv, _ := server.StartMetricsServer(regsvcRegistry, server.RegSvcMetricsPort)
	regsvcSrv := server.New(app)
	err = regsvcSrv.SetupRoutes(proxy.DefaultPort, regsvcRegistry)
	if err != nil {
		panic(err.Error())
	}

	routesToPrint := regsvcSrv.GetRegisteredRoutes()
	log.Infof(nil, "Configured routes: %s", routesToPrint)

	// listen concurrently to allow for graceful shutdown
	go func() {
		log.Infof(nil, "Service Revision %s built on %s", configuration.Commit, configuration.BuildTime)
		log.Infof(nil, "Listening on %q...", configuration.HTTPAddress)
		if err := regsvcSrv.HTTPServer().ListenAndServe(); err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				log.Info(nil, fmt.Sprintf("%s - this is expected when server shutdown has been initiated", err.Error()))
			} else {
				log.Error(nil, err, err.Error())
			}
		}
	}()

	gracefulShutdown(ctx, configuration.GracefulTimeout, regsvcSrv.HTTPServer(), regsvcMetricsSrv, proxySrv, proxyMetricsSrv)
}

func gracefulShutdown(ctx context.Context, timeout time.Duration, hs ...*http.Server) {
	<-ctx.Done()
	// We are done
	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	log.Infof(nil, "Shutdown with timeout: %s", timeout.String())
	for _, s := range hs {
		if err := s.Shutdown(ctxTimeout); err != nil {
			log.Errorf(nil, err, "Shutdown error")
		} else {
			log.Info(nil, "Server stopped.")
		}
	}
}

func newCachedClient(ctx context.Context, cfg *rest.Config) (client.Client, error) {
	scheme := runtime.NewScheme()
	var AddToSchemes runtime.SchemeBuilder
	addToSchemes := append(AddToSchemes,
		corev1.AddToScheme,
		toolchainv1alpha1.AddToScheme)
	err := addToSchemes.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}

	hostCluster, err := runtimecluster.New(cfg, func(options *runtimecluster.Options) {
		options.Scheme = scheme
		// cache only in the host-operator namespace
		options.Namespace = configuration.Namespace()
	})
	if err != nil {
		return nil, err
	}
	go func() {
		if err := hostCluster.Start(ctx); err != nil {
			panic(fmt.Errorf("failed to create cached client: %w", err))
		}
	}()

	if !hostCluster.GetCache().WaitForCacheSync(ctx) {
		return nil, fmt.Errorf("unable to sync the cache of the client")
	}

	// populate the cache backed by shared informers that are initialized lazily on the first call
	// for the given GVK with all resources we are interested in from the host-operator namespace
	objectsToList := map[string]client.ObjectList{
		"MasterUserRecord": &toolchainv1alpha1.MasterUserRecordList{},
		"Space":            &toolchainv1alpha1.SpaceList{},
		"SpaceBinding":     &toolchainv1alpha1.SpaceBindingList{},
		"ToolchainStatus":  &toolchainv1alpha1.ToolchainStatusList{},
		"UserSignup":       &toolchainv1alpha1.UserSignupList{},
		"ProxyPlugin":      &toolchainv1alpha1.ProxyPluginList{},
		"NSTemplateTier":   &toolchainv1alpha1.NSTemplateTierList{},
		"ToolchainConfig":  &toolchainv1alpha1.ToolchainConfigList{},
		"BannedUser":       &toolchainv1alpha1.BannedUserList{},
		"ToolchainCluster": &toolchainv1alpha1.ToolchainClusterList{},
		"Secret":           &corev1.SecretList{}}

	for resourceName := range objectsToList {
		log.Infof(nil, "Syncing informer cache with %s resources", resourceName)
		if err := hostCluster.GetClient().List(ctx, objectsToList[resourceName], client.InNamespace(configuration.Namespace())); err != nil {
			log.Errorf(nil, err, "Informer cache sync failed for %s", resourceName)
			return nil, err
		}
	}

	log.Info(nil, "Informer caches synced")

	return hostCluster.GetClient(), nil
}

func createCaptchaFileFromSecret(cfg configuration.RegistrationServiceConfig) error {
	contents := cfg.Verification().CaptchaServiceAccountFileContents()
	if err := os.WriteFile(configuration.CaptchaFilePath, []byte(contents), 0600); err != nil {
		return errs.Wrap(err, "error writing captcha file")
	}
	return nil
}
