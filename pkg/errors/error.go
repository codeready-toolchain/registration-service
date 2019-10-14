package errors

import (
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
