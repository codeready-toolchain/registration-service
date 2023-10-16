package controller_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	toolchainv1alpha1 "github.com/codeready-toolchain/api/api/v1alpha1"
	"github.com/codeready-toolchain/registration-service/pkg/controller"
	"github.com/codeready-toolchain/registration-service/pkg/username"
	"github.com/codeready-toolchain/registration-service/test"
	"github.com/codeready-toolchain/registration-service/test/fake"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestUsernamesSuite struct {
	test.UnitTestSuite
	httpClient *http.Client
}

func TestRunUsernamesSuite(t *testing.T) {
	suite.Run(t, &TestUsernamesSuite{test.UnitTestSuite{}, nil})
}

func (s *TestUsernamesSuite) TestUsernamesGetHandler() {
	req, err := http.NewRequest(http.MethodGet, "/api/v1/usernames", nil)
	require.NoError(s.T(), err)

	fakeClient := fake.InitClient(s.T(),
		fake.NewMasterUserRecord("johnny"),
	)

	s.Run("success", func() {

		fakeInformer := fake.GetInformerService(fakeClient)()
		s.Application.MockInformerService(fakeInformer)

		// Create Usernames controller instance.
		ctrl := controller.NewUsernames(s.Application)
		handler := gin.HandlerFunc(ctrl.GetHandler)

		s.Run("usernames found", func() {
			// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
			rr := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rr)
			ctx.Request = req
			ctx.AddParam("username", "johnny")
			expected := &username.Response{
				{Username: "johnny"},
			}

			handler(ctx)

			// Check the status code is what we expect.
			assert.Equal(s.T(), http.StatusOK, rr.Code, "handler returned wrong status code")

			// Check the response body is what we expect.
			data := &username.Response{}
			err = json.Unmarshal(rr.Body.Bytes(), &data)
			require.NoError(s.T(), err)

			assert.Equal(s.T(), expected, data)
		})

		s.Run("usernames not found", func() {
			// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
			rr := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rr)
			ctx.Request = req
			ctx.AddParam("username", "noise") // user doesn't exist

			handler(ctx)

			// Check the status code is what we expect.
			assert.Equal(s.T(), http.StatusNotFound, rr.Code, "handler returned wrong status code")
		})

		s.Run("empty query string provided", func() {
			// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
			rr := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rr)
			ctx.Request = req
			ctx.AddParam("username", "") // no username was provided

			handler(ctx)

			// Check the status code is what we expect.
			assert.Equal(s.T(), http.StatusNotFound, rr.Code, "handler returned wrong status code")
		})

	})

	s.Run("error", func() {
		// force error while retrieving MUR
		fakeInformer := fake.GetInformerService(fakeClient, fake.WithGetMurFunc(func(name string) (*toolchainv1alpha1.MasterUserRecord, error) {
			return nil, fmt.Errorf("mock error")
		}))()
		s.Application.MockInformerService(fakeInformer)

		// Create Usernames controller instance.
		ctrl := controller.NewUsernames(s.Application)
		handler := gin.HandlerFunc(ctrl.GetHandler)
		s.Run("unable to get mur", func() {
			// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
			rr := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rr)
			ctx.Request = req
			ctx.AddParam("username", "noise")

			handler(ctx)

			// Check the error is what we expect.
			test.AssertError(s.T(), rr, http.StatusInternalServerError, "mock error", "error getting MasterUserRecord resource")
		})
	})
}
