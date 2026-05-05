package errors_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	errs "github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/codeready-toolchain/registration-service/test"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gotest.tools/assert"
)

type TestErrorsSuite struct {
	test.UnitTestSuite
}

func TestRunErrorsSuite(t *testing.T) {
	suite.Run(t, &TestErrorsSuite{test.UnitTestSuite{}})
}

func (s *TestErrorsSuite) TestErrors() {
	e := echo.New()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := e.NewContext(req, rr)

	s.Run("check json error payload", func() {
		details := "testing payload"
		errMsg := "testing new error"
		code := http.StatusInternalServerError

		err := errs.AbortWithError(ctx, code, errors.New(errMsg), details)
		require.NoError(s.T(), err)

		res := errs.Error{}
		err = json.Unmarshal(rr.Body.Bytes(), &res)
		require.NoError(s.T(), err)

		assert.Equal(s.T(), res.Code, http.StatusInternalServerError)
		assert.Equal(s.T(), res.Details, details)
		assert.Equal(s.T(), res.Message, errMsg)
		assert.Equal(s.T(), res.Status, http.StatusText(code))
	})

	s.Run("check specific error types", func() {
		err := errs.NewForbiddenError("foo", "bar")
		require.Equal(s.T(), "foo", err.Message)
		require.Equal(s.T(), "bar", err.Details)
		require.Equal(s.T(), http.StatusForbidden, err.Code)
		require.Equal(s.T(), http.StatusText(http.StatusForbidden), err.Status)
		require.Equal(s.T(), "foo: bar", err.Error())

		err = errs.NewUnauthorizedError("foo", "bar")
		require.Equal(s.T(), "foo", err.Message)
		require.Equal(s.T(), "bar", err.Details)
		require.Equal(s.T(), http.StatusUnauthorized, err.Code)
		require.Equal(s.T(), http.StatusText(http.StatusUnauthorized), err.Status)
		require.Equal(s.T(), "foo: bar", err.Error())

		err = errs.NewTooManyRequestsError("foo", "bar")
		require.Equal(s.T(), "foo", err.Message)
		require.Equal(s.T(), "bar", err.Details)
		require.Equal(s.T(), http.StatusTooManyRequests, err.Code)
		require.Equal(s.T(), http.StatusText(http.StatusTooManyRequests), err.Status)

		err = errs.NewInternalError(errors.New("some error"), "bar")
		require.Equal(s.T(), "some error", err.Message)
		require.Equal(s.T(), "bar", err.Details)
		require.Equal(s.T(), http.StatusInternalServerError, err.Code)
		require.Equal(s.T(), http.StatusText(http.StatusInternalServerError), err.Status)

		err = errs.NewNotFoundError(errors.New("some error"), "bar")
		require.Equal(s.T(), "some error", err.Message)
		require.Equal(s.T(), "bar", err.Details)
		require.Equal(s.T(), http.StatusNotFound, err.Code)
		require.Equal(s.T(), http.StatusText(http.StatusNotFound), err.Status)

		err = errs.NewBadRequest("foo", "bar")
		require.Equal(s.T(), "foo", err.Message)
		require.Equal(s.T(), "bar", err.Details)
		require.Equal(s.T(), http.StatusBadRequest, err.Code)
		require.Equal(s.T(), http.StatusText(http.StatusBadRequest), err.Status)
	})

	s.Run("StatusCode", func() {
		s.Run("from crterrors.Error", func() {
			require.Equal(s.T(), http.StatusForbidden, errs.StatusCode(errs.NewForbiddenError("a", "b")))
			require.Equal(s.T(), http.StatusNotFound, errs.StatusCode(errs.NewNotFoundError(errors.New("x"), "y")))
		})

		s.Run("from echo.HTTPError", func() {
			require.Equal(s.T(), http.StatusBadGateway, errs.StatusCode(echo.NewHTTPError(http.StatusBadGateway, "bad gateway")))
		})

		s.Run("from unknown error", func() {
			require.Equal(s.T(), http.StatusInternalServerError, errs.StatusCode(errors.New("something went wrong")))
		})
	})
}
