package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	testconfig "github.com/codeready-toolchain/toolchain-common/pkg/test/config"

	"github.com/codeready-toolchain/registration-service/test"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestUIConfigSuite struct {
	test.UnitTestSuite
}

func TestRunUIConfigSuite(t *testing.T) {
	suite.Run(t, &TestUIConfigSuite{test.UnitTestSuite{}})
}

func (s *TestUIConfigSuite) TestHandlersReturnUIConfig() {
	const webhookURL = "https://webhooks.example.com/sandbox"
	s.OverrideApplicationDefault(
		workatoWebHookURL(webhookURL),
		testconfig.RegistrationService().DisabledIntegrations([]string{"openshift", "devspaces"}),
	)
	defer s.DefaultConfig()

	uiConfig := NewUIConfig()

	s.Run("authenticated", func() {
		rr := invokeUIConfig(s.T(), uiConfig.GetHandler, "/api/v1/uiconfig")

		require.Equal(s.T(), http.StatusOK, rr.Code)
		assert.JSONEq(s.T(), `{"workatoWebHookURL":"https://webhooks.example.com/sandbox","disabledIntegrations":["openshift","devspaces"]}`, rr.Body.String())
	})

	s.Run("public", func() {
		rr := invokeUIConfig(s.T(), uiConfig.GetPublicHandler, "/api/v1/uiconfig/public")

		require.Equal(s.T(), http.StatusOK, rr.Code)
		assert.JSONEq(s.T(), `{"disabledIntegrations":["openshift","devspaces"]}`, rr.Body.String())
	})
}

func (s *TestUIConfigSuite) TestHandlersReturnEmptyDefaults() {
	uiConfig := NewUIConfig()

	s.Run("authenticated", func() {
		rr := invokeUIConfig(s.T(), uiConfig.GetHandler, "/api/v1/uiconfig")

		require.Equal(s.T(), http.StatusOK, rr.Code)
		assert.JSONEq(s.T(), `{"workatoWebHookURL":"","disabledIntegrations":[]}`, rr.Body.String())
	})

	s.Run("public", func() {
		rr := invokeUIConfig(s.T(), uiConfig.GetPublicHandler, "/api/v1/uiconfig/public")

		require.Equal(s.T(), http.StatusOK, rr.Code)
		assert.JSONEq(s.T(), `{"disabledIntegrations":[]}`, rr.Body.String())
	})
}

func invokeUIConfig(t *testing.T, handler gin.HandlerFunc, path string) *httptest.ResponseRecorder {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, path, nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rr)
	ctx.Request = req
	handler(ctx)
	return rr
}

type workatoWebHookURL string

func (o workatoWebHookURL) Apply(config *toolchainv1alpha1.ToolchainConfig) {
	url := string(o)
	config.Spec.Host.RegistrationService.WorkatoWebHookURL = &url
}
