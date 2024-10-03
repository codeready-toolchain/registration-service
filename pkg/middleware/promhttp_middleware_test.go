package middleware_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/proxy"
	"github.com/codeready-toolchain/registration-service/pkg/server"
	"github.com/codeready-toolchain/registration-service/test"
	"github.com/codeready-toolchain/registration-service/test/fake"
	authsupport "github.com/codeready-toolchain/toolchain-common/pkg/test/auth"
	testconfig "github.com/codeready-toolchain/toolchain-common/pkg/test/config"

	"github.com/prometheus/client_golang/prometheus"
	prommodel "github.com/prometheus/client_model/go"
	promcommon "github.com/prometheus/common/expfmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type PromHTTPMiddlewareSuite struct {
	test.UnitTestSuite
}

func TestPromHTTPMiddlewareSuite(t *testing.T) {
	suite.Run(t, &PromHTTPMiddlewareSuite{test.UnitTestSuite{}})
}

func (s *PromHTTPMiddlewareSuite) TestPromHTTPMiddleware() {
	// given
	tokengenerator := authsupport.NewTokenManager()
	srv := server.New(fake.NewMockableApplication())
	keysEndpointURL := tokengenerator.NewKeyServer().URL
	s.SetConfig(testconfig.RegistrationService().
		Environment(configuration.UnitTestsEnvironment).
		Auth().AuthClientPublicKeysURL(keysEndpointURL))

	cfg := configuration.GetRegistrationServiceConfig()
	assert.Equal(s.T(), keysEndpointURL, cfg.Auth().AuthClientPublicKeysURL(), "key url not set correctly")
	reg := prometheus.NewRegistry()
	err := srv.SetupRoutes(proxy.DefaultPort, reg)
	require.NoError(s.T(), err)

	// making a call on an HTTP endpoint
	resp := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/api/v1/segment-write-key", nil)
	require.NoError(s.T(), err)

	// when
	srv.Engine().ServeHTTP(resp, req)

	// then
	assert.Equal(s.T(), http.StatusOK, resp.Code, "request returned wrong status code")

	s.Run("check metrics", func() {
		// setup the metrics server to access the Prometheus registry contents
		_, router := server.StartMetricsServer(reg, server.RegSvcMetricsPort)

		resp = httptest.NewRecorder()
		req, err = http.NewRequest(http.MethodGet, "/metrics", nil)
		require.NoError(s.T(), err)

		// when
		router.ServeHTTP(resp, req)

		// then
		assert.Equal(s.T(), http.StatusOK, resp.Code, "request returned wrong status code")
		require.NoError(s.T(), err)

		assertMetricExists(s.T(), resp.Body.Bytes(), "sandbox_promhttp_client_in_flight_requests", nil)
		assertMetricExists(s.T(), resp.Body.Bytes(), "sandbox_promhttp_client_api_requests_total", map[string]string{
			"code":   "200",
			"method": "GET",
			"path":   "/api/v1/segment-write-key",
		})
		assertMetricExists(s.T(), resp.Body.Bytes(), "sandbox_promhttp_request_duration_seconds", map[string]string{
			"code":   "200",
			"method": "GET",
			"path":   "/api/v1/segment-write-key",
		})
	})
}

func assertMetricExists(t *testing.T, data []byte, name string, labels map[string]string) {
	p := &promcommon.TextParser{}
	metrics, err := p.TextToMetricFamilies(bytes.NewReader(data))
	require.NoError(t, err)

	metric, found := metrics[name]
	require.True(t, found, "unable to find metric '%s'", name)
	for _, m := range metric.GetMetric() {
		if matchesLabels(m.GetLabel(), labels) {
			return
		}
	}
	assert.Fail(t, "unable to find metric '%s' with labels '%v'", name, labels)
}

func matchesLabels(x []*prommodel.LabelPair, y map[string]string) bool {
	if len(x) != len(y) {
		return false
	}
	for _, l := range x {
		if v, found := y[l.GetName()]; !found || l.GetValue() != v {
			return false
		}
	}
	return true
}
