package errors

import (
	"fmt"
	errors2 "k8s.io/apimachinery/pkg/api/errors"
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

// AbortWithStatusError stops the chain, writes the status code and the given error
func AbortWithStatusError(ctx *gin.Context, err error, details string) {
	code := http.StatusInternalServerError
	if statusError, ok := err.(*errors2.StatusError); ok {
		code = int(statusError.Status().Code)
	}

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