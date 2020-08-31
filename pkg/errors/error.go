package errors

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Error struct {
	Status  string `json:"status"`
	Code    int    `json:"code"`
	Message string `json:"message"`
	Details string `json:"details"`
}

// AbortWithError stops the chain, writes the status code and the given error
func AbortWithError(ctx *gin.Context, code int, err error, details string) {
	ctx.AbortWithStatusJSON(code, &Error{
		Status:  http.StatusText(code),
		Code:    code,
		Message: err.Error(),
		Details: details,
	})
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s:%s", e.Message, e.Details)
}

func NewForbiddenError(message, details string) *Error {
	return &Error{
		Status:  http.StatusText(http.StatusForbidden),
		Code:    http.StatusForbidden,
		Message: message,
		Details: details,
	}
}

func NewTooManyRequestsError(message, details string) *Error {
	return &Error{
		Status:  http.StatusText(http.StatusTooManyRequests),
		Code:    http.StatusTooManyRequests,
		Message: message,
		Details: details,
	}
}

func NewInternalError(err error, details string) *Error {
	return &Error{
		Status:  http.StatusText(http.StatusInternalServerError),
		Code:    http.StatusInternalServerError,
		Message: err.Error(),
		Details: details,
	}
}

func NewNotFoundError(err error, details string) *Error {
	return &Error{
		Status:  http.StatusText(http.StatusNotFound),
		Code:    http.StatusNotFound,
		Message: err.Error(),
		Details: details,
	}
}
