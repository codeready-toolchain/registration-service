package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/namespaces"
	"github.com/codeready-toolchain/registration-service/test"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TestNamespacesSuite struct {
	test.UnitTestSuite
}

func TestRunNamespacesSuite(t *testing.T) {
	suite.Run(t, &TestNamespacesSuite{test.UnitTestSuite{}})
}

// mockNamespacesManager mocks the "NamespacesManager" interface and allows
// us to return custom return values from it, to effectively unit test the
// handler in isolation.
type mockNamespacesManager struct {
	ResetNamespacesReturnValue error
}

func (mnm *mockNamespacesManager) ResetNamespaces(_ *gin.Context) error {
	return mnm.ResetNamespacesReturnValue
}

// TestResetNamespacesHandler tests that the handler returns a proper
// non-error response when the operation succeeds, and that it returns a
// structured error with an explanation in the opposite case.
func (ns *TestNamespacesSuite) TestResetNamespacesHandler() {
	ns.Run(`handler returns "Accepted" response when no errors occur`, func() {
		// given
		// Prepare the request and the context for gin.
		req, err := http.NewRequest(http.MethodPost, "/api/v1/reset-namespaces", nil)
		if err != nil {
			ns.Fail("unable to create test request", err.Error())
			return
		}

		// Prepare the handler under test.
		mnm := mockNamespacesManager{}
		ctrl := NewNamespacesController(&mnm)
		handler := gin.HandlerFunc(ctrl.ResetNamespaces)

		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		// when
		// Call the handler under test.
		handler(ctx)

		// then
		// Assert that the correct status code was returned. The namespace
		// manager will not return any errors, so we expect a proper status
		// code from the handler.
		require.Equal(ns.T(), http.StatusAccepted, rr.Code)
	})

	ns.Run(`handler returns a "Not Found" error when the user signup cannot be found or is deactivated`, func() {
		// given
		// Prepare the request and the context for gin.
		req, err := http.NewRequest(http.MethodPost, "/api/v1/reset-namespaces", nil)
		if err != nil {
			ns.Fail("unable to create test request", err.Error())
			return
		}

		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		// Override the namespace manager's response, to simulate that the
		// user signup could not be found or is deactivated.
		mnm := mockNamespacesManager{}
		ctrl := NewNamespacesController(&mnm)
		handler := gin.HandlerFunc(ctrl.ResetNamespaces)

		mnm.ResetNamespacesReturnValue = namespaces.ErrUserSignUpNotFoundDeactivated{}

		// when
		// Call the handler under test.
		handler(ctx)

		// then
		// Assert that the proper "Not Found" error response was returned.
		userError := NamespaceResetError{}
		test.AssertError(ns.T(), rr, http.StatusNotFound, userError.Error(), "The user is either not found or deactivated. Please contact the Developer Sandbox team at developersandbox@redhat.com for assistance")
	})

	ns.Run(`handler returns a "Bad Request" error when the user does not have any provisioned namespaces`, func() {
		// given
		// Prepare the request and the context for gin.
		req, err := http.NewRequest(http.MethodPost, "/api/v1/reset-namespaces", nil)
		if err != nil {
			ns.Fail("unable to create test request", err.Error())
			return
		}

		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		// Override the namespace manager's response, to simulate that the
		// user does not have any provisioned namespaces, and therefore the
		// manager returns the corresponding error which should be handled
		// properly by the handler.
		mnm := mockNamespacesManager{}
		ctrl := NewNamespacesController(&mnm)
		handler := gin.HandlerFunc(ctrl.ResetNamespaces)

		mnm.ResetNamespacesReturnValue = namespaces.ErrUserHasNoProvisionedNamespaces{}

		// when
		// Call the handler under test.
		handler(ctx)

		// then
		// Assert that the proper "Bad Request" error response was returned.
		userError := NamespaceResetError{}
		test.AssertError(ns.T(), rr, http.StatusBadRequest, userError.Error(), "No namespaces provisioned, unable to perform reset. Please try again in a while and if the issue persists, please contact the Developer Sandbox team at developersandbox@redhat.com for assistance")
	})

	ns.Run(`handler returns an internal server error when a generic error occurs`, func() {
		// given
		// Prepare the request and the context for gin.
		req, err := http.NewRequest(http.MethodPost, "/api/v1/reset-namespaces", nil)
		if err != nil {
			ns.Fail("unable to create test request", err.Error())
			return
		}

		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)
		ctx.Request = req

		// Override the namespace manager's response, to simulate an error when
		// attempting to delete the namespaces. We recreate the manager so that
		// we don't have test failures in case the test suite is run in
		// parallel.
		mnm := mockNamespacesManager{}
		ctrl := NewNamespacesController(&mnm)
		handler := gin.HandlerFunc(ctrl.ResetNamespaces)

		mnm.ResetNamespacesReturnValue = errors.New("test error")

		// when
		// Call the handler under test.
		handler(ctx)

		// then
		// Assert that the proper "Internal Server Error" error response was
		// returned.
		userError := NamespaceResetError{}
		test.AssertError(ns.T(), rr, http.StatusInternalServerError, userError.Error(), "Unable to reset your namespaces. Please try again in a while and if the issue persists, please contact the Developer Sandbox team at developersandbox@redhat.com for assistance")
	})
}
