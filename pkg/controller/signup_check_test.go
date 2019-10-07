package controller_test

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"time"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/controller"
	testutils "github.com/codeready-toolchain/registration-service/test"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestSignupCheckSuite struct {
	testutils.UnitTestSuite
}

func TestRunSignupCheckSuite(t *testing.T) {
	suite.Run(t, &TestSignupCheckSuite{testutils.UnitTestSuite{}})
}

func (s *TestSignupCheckSuite) TestSignupCheckHandler() {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest(http.MethodGet, "/api/v1/signupcheck", nil)
	require.NoError(s.T(), err)

	// Create logger and registry.
	logger := log.New(os.Stderr, "", 0)

	// Check if the config is set to testing mode, so the handler may use this.
	assert.True(s.T(), s.Config.IsTestingMode(), "testing mode not set correctly to true")

	// Create SignupCheck check instance.
	SignupCheckCtrl := controller.NewSignupCheck(logger, s.Config)
	handler := gin.HandlerFunc(SignupCheckCtrl.GetHandler)

	s.Run("SignupCheck in testing mode", func() {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		handler(ctx)

		// Check the status code is what we expect.
		assert.Equal(s.T(), rr.Code, http.StatusOK, "handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)

		// Check the response body is what we expect.
		data := &controller.SignupCheckPayload{}
		err := json.Unmarshal(rr.Body.Bytes(), &data)
		require.NoError(s.T(), err)

		val := data.ProvisioningDone
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
			ctx.Request = req
			handler(ctx)
			assert.Equal(s.T(), rr.Code, http.StatusOK, "handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
			data := &controller.SignupCheckPayload{}
			err := json.Unmarshal(rr.Body.Bytes(), &data)
			require.NoError(s.T(), err)
			if time.Now().Unix() < testStartTimestamp+5 {
				assert.False(s.T(), data.ProvisioningDone, "ProvisioningDone is true before 10s in test mode signupcheck response")			
			} else {
				assert.True(s.T(), data.ProvisioningDone, "ProvisioningDone is false after 10s in test mode signupcheck response")			
			}
			time.Sleep(2 * time.Second)
		}
	})
}
