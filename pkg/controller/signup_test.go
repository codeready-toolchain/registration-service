package controller_test

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/controller"
	"github.com/codeready-toolchain/registration-service/pkg/middleware"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	testutils "github.com/codeready-toolchain/registration-service/test"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

var testRequestTimestamp int64

type TestSignupSuite struct {
	testutils.UnitTestSuite
}

func TestRunSignupSuite(t *testing.T) {
	suite.Run(t, &TestSignupSuite{testutils.UnitTestSuite{}})
}

func (s *TestSignupSuite) TestSignupPostHandler() {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest(http.MethodPost, "/api/v1/signup", nil)
	require.NoError(s.T(), err)

	// Create logger and registry.
	logger := log.New(os.Stderr, "", 0)

	// Check if the config is set to testing mode, so the handler may use this.
	assert.True(s.T(), s.Config.IsTestingMode(), "testing mode not set correctly to true")

	// Create signup instance.
	signupCtrl := controller.NewSignup(logger, s.Config, nil)
	handler := gin.HandlerFunc(signupCtrl.PostHandler)

	s.Run("signup", func() {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		handler(ctx)

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusOK, rr.Code)
	})
}

// getTestSignupCheckInfo retrieves a test check info. Used only for tests.
// It reports provisioning/not ready for 5s, then reports state complete.
func getTestSignupCheckInfo(userID string) (*signup.Signup, error) {
	payload := &signup.Signup{
		TargetCluster: "clusterX",
		Username:      "userID",
		Status: signup.SignupStatus{
			Ready:   true,
			Reason:  "",
			Message: "",
		},
	}
	if testRequestTimestamp == 0 {
		testRequestTimestamp = time.Now().Unix()
	}
	if time.Now().Unix()-testRequestTimestamp >= 5 {
		payload.Status.Ready = true
		payload.Status.Reason = "provisioned"
		payload.Status.Message = "testing mode - done"
	} else {
		payload.Status.Ready = false
		payload.Status.Reason = "provisioning"
		payload.Status.Message = "testing mode - waiting for timeout"
	}
	return payload, nil
}

func (s *TestSignupSuite) TestSignupGetHandler() {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest(http.MethodGet, "/api/v1/signup", nil)
	require.NoError(s.T(), err)

	// Create logger and registry.
	logger := log.New(os.Stderr, "", 0)

	// Check if the config is set to testing mode, so the handler may use this.
	assert.True(s.T(), s.Config.IsTestingMode(), "testing mode not set correctly to true")
	// Add the mock checker func to the config (this is only used in testing, so accessing the Viper manually)
	s.Config.GetViperInstance().Set("checker", getTestSignupCheckInfo)

	// Create SignupCheck check instance.
	signupSrv, err := signup.NewSignupService(s.Config)
	require.NoError(s.T(), err)
	SignupCheckCtrl := controller.NewSignup(logger, s.Config, signupSrv)
	handler := gin.HandlerFunc(SignupCheckCtrl.GetHandler)

	s.Run("SignupCheck in testing mode", func() {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		ctx.Set(middleware.SubKey, "sub-value")
		ctx.Set(middleware.EmailKey, "email@email.email")
		ctx.Set(middleware.UsernameKey, "username-value")
		handler(ctx)

		// Check the status code is what we expect.
		assert.Equal(s.T(), rr.Code, http.StatusOK, "handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)

		// Check the response body is what we expect.
		data := &signup.Signup{}
		err := json.Unmarshal(rr.Body.Bytes(), &data)
		require.NoError(s.T(), err)

		val := data.Status.Ready
		assert.False(s.T(), val, "ProvisioningDone is true in test mode signupcheck initial response")
	})

	s.Run("ProvisioningDone in testing mode", func() {
		testStartTimestamp := time.Now().Unix()
		log.Printf("TIME1 %d", time.Now().Unix())
		log.Printf("TIME2 %d", time.Now().Unix())
		// do a few requests every 2 seconds, with the requests after elapsed 5s returning ProvisioningDone==true.
		for time.Now().Unix() < testStartTimestamp+10 {
			rr := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rr)
			ctx.Set(middleware.SubKey, "sub-value")
			ctx.Set(middleware.EmailKey, "email@email.email")
			ctx.Set(middleware.UsernameKey, "username-value")
			ctx.Request = req
			handler(ctx)
			assert.Equal(s.T(), rr.Code, http.StatusOK, "handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
			data := &signup.Signup{}
			err := json.Unmarshal(rr.Body.Bytes(), &data)
			require.NoError(s.T(), err)
			if time.Now().Unix() < testStartTimestamp+5 {
				assert.False(s.T(), data.Status.Ready, "ProvisioningDone is true before 10s in test mode signupcheck response")
				assert.Equal(s.T(), "provisioning", data.Status.Reason)
				assert.Equal(s.T(), "testing mode - waiting for timeout", data.Status.Message)
			} else {
				assert.True(s.T(), data.Status.Ready, "ProvisioningDone is false after 10s in test mode signupcheck response")
				assert.Equal(s.T(), "provisioned", data.Status.Reason)
				assert.Equal(s.T(), "testing mode - done", data.Status.Message)
			}
			time.Sleep(2 * time.Second)
		}
	})
}
