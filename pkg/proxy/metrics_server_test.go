package proxy_test

import (
	"bytes"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/proxy"
	"github.com/codeready-toolchain/registration-service/pkg/proxy/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProxyMetricsServer(t *testing.T) {
	reg := prometheus.NewRegistry()
	_ = metrics.NewProxyMetrics(reg)
	server := proxy.StartMetricsServer(reg, proxy.ProxyMetricsPort)
	require.NotNil(t, server)
	// Wait up to N seconds for the Metrics server to start
	ready := false
	sec := 10
	for i := 0; i < sec; i++ {
		req, err := http.NewRequest("GET", "http://localhost:8082/metrics", nil)
		require.NoError(t, err)
		require.NotNil(t, req)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			// The server may be running but still not fully ready to accept requests
			time.Sleep(time.Second)
			continue
		}
		// Server is up and running!
		ready = true
		break
	}
	require.True(t, ready, "Metrics Server is not ready after %d seconds", sec)
	defer func() {
		_ = server.Close()
	}()

	req, err := http.NewRequest("GET", "http://localhost:8082/metrics", nil)
	require.NoError(t, err)
	require.NotNil(t, req)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "text/plain; version=0.0.4; charset=utf-8; escaping=underscores", resp.Header.Get("Content-Type"))
	// compare the body of the response as well
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, expectedServerBlankResponse, buf.String())
}

var expectedServerBlankResponse = `# HELP promhttp_metric_handler_errors_total Total number of internal errors encountered by the promhttp metric handler.
# TYPE promhttp_metric_handler_errors_total counter
promhttp_metric_handler_errors_total{cause="encoding"} 0
promhttp_metric_handler_errors_total{cause="gathering"} 0
`
