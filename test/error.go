package test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AssertError asserts that the provided response contains the expected error
func AssertError(t *testing.T, actualResponse *httptest.ResponseRecorder, expectedErrorCode int, expectedMessageAndDetails ...string) {
	// Check the status code is what we expect.
	assert.Equal(t, expectedErrorCode, actualResponse.Code, "handler returned wrong status code")

	// Check the response body is what we expect.
	data := &errors.Error{}
	err := json.Unmarshal(actualResponse.Body.Bytes(), &data)
	require.NoError(t, err)

	var message, details string
	if len(expectedMessageAndDetails) > 0 {
		message = expectedMessageAndDetails[0]
	}
	if len(expectedMessageAndDetails) > 0 {
		details = expectedMessageAndDetails[1]
	}
	assert.Equal(t, &errors.Error{
		Status:  http.StatusText(expectedErrorCode),
		Code:    expectedErrorCode,
		Message: message,
		Details: details,
	}, data)
}
