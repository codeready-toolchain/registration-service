package errors_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	errs "github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/codeready-toolchain/registration-service/test"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gotest.tools/assert"
	errors2 "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type TestErrorsSuite struct {
	test.UnitTestSuite
}

func TestRunErrorsSuite(t *testing.T) {
	suite.Run(t, &TestErrorsSuite{test.UnitTestSuite{}})
}

func (s *TestErrorsSuite) TestErrors() {
	s.Run("test AbortWithError", func() {
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)

		details := "testing payload"
		errMsg := "testing new error"
		code := http.StatusInternalServerError

		errs.AbortWithError(ctx, code, errors.New(errMsg), details)

		res := errs.Error{}
		err := json.Unmarshal(rr.Body.Bytes(), &res)
		require.NoError(s.T(), err)

		assert.Equal(s.T(), res.Code, http.StatusInternalServerError)
		assert.Equal(s.T(), res.Details, details)
		assert.Equal(s.T(), res.Message, errMsg)
		assert.Equal(s.T(), res.Status, http.StatusText(code))
	})
	s.Run("test AbortWithStatusError", func() {
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)

		details := "testing payload"
		errMsg := "testing new error"

		errs.AbortWithStatusError(ctx, errors.New(errMsg), details)

		res := errs.Error{}
		err := json.Unmarshal(rr.Body.Bytes(), &res)
		require.NoError(s.T(), err)

		assert.Equal(s.T(), res.Code, http.StatusInternalServerError)
		assert.Equal(s.T(), res.Details, details)
		assert.Equal(s.T(), res.Message, errMsg)
		assert.Equal(s.T(), res.Status, http.StatusText(http.StatusInternalServerError))
	})
	s.Run("test AbortWithStatusError bad request", func() {
		rr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rr)

		details := "testing payload"
		errMsg := "testing new error"
		badReqError := &errors2.StatusError{metav1.Status{
			Message: errMsg,
			Reason:  metav1.StatusReason(details),
			Code:    400,
		}}
		errs.AbortWithStatusError(ctx, badReqError, details)

		res := errs.Error{}
		err := json.Unmarshal(rr.Body.Bytes(), &res)
		require.NoError(s.T(), err)

		assert.Equal(s.T(), res.Code, http.StatusBadRequest)
		assert.Equal(s.T(), res.Details, details)
		assert.Equal(s.T(), res.Message, errMsg)
		assert.Equal(s.T(), res.Status, http.StatusText(http.StatusBadRequest))
	})
}
