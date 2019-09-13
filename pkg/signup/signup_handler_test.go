package signup_test

import (
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignupHandler(t *testing.T) {
	// Create logger and registry.
	logger := log.New(os.Stderr, "", 0)
	configRegistry := configuration.CreateEmptyRegistry()

	// test key
	keyJSON := `{"keys":[
		{"kid":"key1","kty":"RSA","e":"AQAB","n":"4niTFsMZ_gLOcg9OuwMK4LpBzpdS8ulIGmx5B4rNqWVHAWMpg4kEmZTQffVmKmiw3NUDSaSWLcLJp22ekN2sj1E7tEu1pJksYsXNDa3WLaE1uqVeso-HVv2rIbucd5xMaryvf490g2I-PSrZdSvN73VqJM525s7pPanxe1skqh8"}
	]}`
	// test JWT, signed with key1
	jwt0 := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6ImtleTEifQ.eyJqdGkiOiIwMjgyYjI5Yy01MTczLTQyZDgtODE0NS1iNDVmYTFlMzUzOGMiLCJleHAiOjAsIm5iZiI6MCwiaWF0IjoxNTE3MDE1OTUyLCJpc3MiOiJ0ZXN0IiwiYXVkIjoiY29kZXJlYWR5LXJlZ2lzdHJhdGlvbi1zZXJ2aWNlIiwic3ViIjoiMjM5ODQzOTgtODU1YS00MmQ2LWE3ZmUtOTM2YmI0ZTkyYTBkIiwidHlwIjoiQmVhcmVyIiwic2Vzc2lvbl9zdGF0ZSI6ImVhZGMwNjZjLTEyMzQtNGE1Ni05ZjM1LWNlNzA3YjU3YTRlMCIsImFjciI6IjAiLCJhbGxvd2VkLW9yaWdpbnMiOlsiKiJdLCJhcHByb3ZlZCI6dHJ1ZSwiZW1haWxfdmVyaWZpZWQiOnRydWUsIm5hbWUiOiJUZXN0MSBVc2VyMSIsImNvbXBhbnkiOiJUZXN0IENvbXBhbnkgMSIsInByZWZlcnJlZF91c2VybmFtZSI6InRlc3R1c2VyMSIsImdpdmVuX25hbWUiOiJUZXN0MSIsImZhbWlseV9uYW1lIjoiVXNlcjEiLCJlbWFpbCI6InRlc3R1c2VyMUB0ZXN0LnQifQ.XoLc1_ESvotJwwfK-zsyu4wySeFalGuHWB1cHRVBPKmztOPgQUGOb4zplhyKAuPPf0x3WGV50ESNW9t-YZoD-DJceMJY_AOzt0LBAoGoeovsVHbrHNGgMDEJsgjQd_beMQsqVGeJrReN9hnqZ8iPz1itMzzTUskG1TQylcr_ez4"
	// jwt1 is signed with key1, but has a different set of payload claims
	jwt1 := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6ImtleTEifQ.eyJqdGkiOiIwMjgyYjI5Yy01MTczLTQyZDgtODE0NS1iNDVmYTFlMzUzOGYiLCJleHAiOjAsIm5iZiI6MCwiaWF0IjoxNTE3MDE1OTUyLCJpc3MiOiJ0ZXN0IiwiYXVkIjoiY29kZXJlYWR5LXJlZ2lzdHJhdGlvbi1zZXJ2aWNlIiwic3ViIjoiMjM5ODQzOTgtODU1YS00MmQ2LWE3ZmUtOTM2YmI0ZTkyYTBmIiwidHlwIjoiQmVhcmVyIiwic2Vzc2lvbl9zdGF0ZSI6ImVhZGMwNjZjLTEyMzQtNGE1Ni05ZjM1LWNlNzA3YjU3YTRlYSIsImFjciI6IjAiLCJhbGxvd2VkLW9yaWdpbnMiOlsiKiJdLCJlbWFpbGFkZHJlc3MiOiJ0ZXN0dXNlcjJAdGVzdC50In0.vg3FVg-e6GOfpWeqoM4BytbCjORSduNGNZzVnqX5Tg1ObqFxf_m-n0fXko3_F4vN_llPeaD_aqvVA-e2k-RzbGqIqgr3IzTWpuedZJzrxln05QQDZiHxpR8mYczQpLQ0BCrLCDsi26iVHEl8L08qAML4npxUlb_OQuqLpZ5IdMA"

	// Set the config for testing mode, the handler may use this.
	configRegistry.GetViperInstance().Set("testingmode", true)
	configRegistry.GetViperInstance().Set("testkeys", keyJSON)
	assert.True(t, configRegistry.IsTestingMode(), "testing mode not set correctly to true")

	// Create handler instance.
	signupService, err := signup.NewSignupService(logger, configRegistry)
	require.NoError(t, err)
	handler := gin.HandlerFunc(signupService.PostSignupHandler)

	t.Run("signup with valid JWT", func(t *testing.T) {
		// add form-data for the JWT to the request
		form := url.Values{}
		form.Add("jwt", jwt0)
		// Create a request to pass to our handler.
		req, err := http.NewRequest(http.MethodPost, "/api/v1/signup", strings.NewReader(form.Encode()))
		require.NoError(t, err)
		// set correct content type
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req
		handler(ctx)
		// Check the status code is what we expect.
		require.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("signup with invalid JWT", func(t *testing.T) {
		// add form-data for the JWT to the request
		form := url.Values{}
		form.Add("jwt", jwt1)
		// Create a request to pass to our handler.
		req, err := http.NewRequest(http.MethodPost, "/api/v1/signup", strings.NewReader(form.Encode()))
		require.NoError(t, err)
		// set correct content type
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req
		handler(ctx)
		// Check the status code is what we expect.
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})
}
