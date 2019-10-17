package controller_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	crtapi "github.com/codeready-toolchain/api/pkg/apis/toolchain/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/context"
	"github.com/codeready-toolchain/registration-service/pkg/controller"
	errs "github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/codeready-toolchain/registration-service/pkg/signup"
	testutils "github.com/codeready-toolchain/registration-service/test"
	"github.com/gofrs/uuid"

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

	// Check if the config is set to testing mode, so the handler may use this.
	assert.True(s.T(), s.Config.IsTestingMode(), "testing mode not set correctly to true")

	// Create a mock SignupService
	svc := &FakeSignupService{}

	// Create signup instance.
	signupCtrl := controller.NewSignup(s.Config, svc)
	handler := gin.HandlerFunc(signupCtrl.PostHandler)

	s.Run("signup", func() {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		svc.MockCreateUserSignup = func(username, userID string) (*crtapi.UserSignup, error) {
			value := &crtapi.UserSignup{}
			return value, nil
		}

		handler(ctx)

		// Check the status code is what we expect.
		require.Equal(s.T(), http.StatusOK, rr.Code)
	})
}

func (s *TestSignupSuite) TestSignupGetHandler() {
	// Create a request to pass to our handler. We don't have any query parameters for now, so we'll
	// pass 'nil' as the third parameter.
	req, err := http.NewRequest(http.MethodGet, "/api/v1/signup", nil)
	require.NoError(s.T(), err)

	// Create a mock SignupService
	svc := &FakeSignupService{}
	// Create UserSignup
	ob, err := uuid.NewV4()
	require.NoError(s.T(), err)
	userID := ob.String()

	// Create Signup controller instance.
	ctrl := controller.NewSignup(s.Config, svc)
	handler := gin.HandlerFunc(ctrl.GetHandler)

	s.Run("signups found", func() {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req
		ctx.Set(context.SubKey, userID)

		targetCluster, err := uuid.NewV4()
		require.NoError(s.T(), err)

		expected := &signup.Signup{
			TargetCluster: "cluster-" + targetCluster.String(),
			Username:      "jsmith",
			Status: signup.Status{
				Reason: "Provisioning",
			},
		}
		svc.MockGetSignup = func(id string) (*signup.Signup, error) {
			if id == userID {
				return expected, nil
			}
			return nil, nil
		}

		handler(ctx)

		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusOK, rr.Code, "handler returned wrong status code")

		// Check the response body is what we expect.
		data := &signup.Signup{}
		err = json.Unmarshal(rr.Body.Bytes(), &data)
		require.NoError(s.T(), err)

		assert.Equal(s.T(), expected, data)
	})

	s.Run("signups not found", func() {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req
		ctx.Set(context.SubKey, userID)

		svc.MockGetSignup = func(id string) (*signup.Signup, error) {
			return nil, nil
		}

		handler(ctx)

		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusNotFound, rr.Code, "handler returned wrong status code")
	})

	s.Run("signups service error", func() {
		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req
		ctx.Set(context.SubKey, userID)

		svc.MockGetSignup = func(id string) (*signup.Signup, error) {
			return nil, errors.New("oopsie woopsie")
		}

		handler(ctx)

		// Check the status code is what we expect.
		assert.Equal(s.T(), http.StatusInternalServerError, rr.Code, "handler returned wrong status code")

		// Check the response body is what we expect.
		data := &errs.Error{}
		err := json.Unmarshal(rr.Body.Bytes(), &data)
		require.NoError(s.T(), err)

		assert.Equal(s.T(), &errs.Error{
			Status:  http.StatusText(http.StatusInternalServerError),
			Code:    http.StatusInternalServerError,
			Message: "oopsie woopsie",
			Details: "error getting UserSignup resource",
		}, data)
	})
}

type FakeSignupService struct {
	MockGetSignup        func(userID string) (*signup.Signup, error)
	MockCreateUserSignup func(username, userID string) (*crtapi.UserSignup, error)
}

func (m *FakeSignupService) GetSignup(userID string) (*signup.Signup, error) {
	return m.MockGetSignup(userID)
}

func (m *FakeSignupService) CreateUserSignup(username, userID string) (*crtapi.UserSignup, error) {
	return m.MockCreateUserSignup(username, userID)
}
