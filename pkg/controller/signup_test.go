package controller_test

import (
	//"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/controller"
	testutils "github.com/codeready-toolchain/registration-service/test"

	/*
	"github.com/codeready-toolchain/registration-service/pkg/middleware"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	"github.com/codeready-toolchain/registration-service/test/fake"
	"github.com/gofrs/uuid"
	apiv1 "k8s.io/api/core/v1"
	*/

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

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

/* TODO: this need to be re-enabled when the GetSignup stuff works

func (s *TestSignupSuite) TestSignupGetHandler() {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest(http.MethodGet, "/api/v1/signup", nil)
	require.NoError(s.T(), err)

	// Create logger and registry.
	logger := log.New(os.Stderr, "", 0)

	// Check if the config is set to testing mode, so the handler may use this.
	assert.True(s.T(), s.Config.IsTestingMode(), "testing mode not set correctly to true")

	// Create a mock SignupService
	svc := &signup.ServiceImpl{
		Namespace:   "test-namespace-123",
		UserSignups: fake.NewFakeUserSignupClient("test-namespace-123"),
		MasterUserRecords: fake.NewFakeMasterUserRecordClient("test-namespace-123"),
	}
	// Create UserSignup
	userID, err := uuid.NewV4()
	require.NoError(s.T(), err)
	userSignup, err := svc.CreateUserSignup("jsmith", userID.String())
	require.NoError(s.T(), err)
	require.NotNil(s.T(), userSignup)

	// Create SignupCheck check instance.
	SignupCheckCtrl := controller.NewSignup(logger, s.Config, svc)
	handler := gin.HandlerFunc(SignupCheckCtrl.GetHandler)

	s.Run("SignupCheck for not ready signups", func() {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		ctx.Set(middleware.SubKey, userID.String())
		ctx.Set(middleware.EmailKey, "email@email.email")
		ctx.Set(middleware.UsernameKey, "jsmith")
		handler(ctx)

		// Check the status code is what we expect.
		assert.Equal(s.T(), rr.Code, http.StatusOK, "handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)

		log.Println(string(rr.Body.Bytes()))

		// Check the response body is what we expect.
		data := &signup.Signup{}
		err := json.Unmarshal(rr.Body.Bytes(), &data)
		require.NoError(s.T(), err)

		val := data.Status.Ready
		assert.Equal(s.T(), apiv1.ConditionFalse, val, "ProvisioningDone is true in test mode signupcheck initial response")
	})

	s.Run("SignupCheck for ready signups", func() {
		// Set mock signup to ready
		// TODO: this needs to be implemented

		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		ctx.Set(middleware.SubKey, userID.String())
		ctx.Set(middleware.EmailKey, "email@email.email")
		ctx.Set(middleware.UsernameKey, "jsmith")
		handler(ctx)

		// Check the status code is what we expect.
		assert.Equal(s.T(), rr.Code, http.StatusOK, "handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)

		// Check the response body is what we expect.
		data := &signup.Signup{}
		err := json.Unmarshal(rr.Body.Bytes(), &data)
		require.NoError(s.T(), err)

		val := data.Status.Ready
		assert.Equal(s.T(), apiv1.ConditionFalse, val, "ProvisioningDone is true in test mode signupcheck initial response")
	})
}
*/