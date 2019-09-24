package errors

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Error struct {
	Status  string
	Code    int
	Message string
	Details string
}

// EncodeError encodes a json error response.
func EncodeError(ctx *gin.Context, err error, code int, details string) {
	// construct an error.
	errorStruct := &Error{
		Status:  http.StatusText(code),
		Code:    code,
		Message: err.Error(),
		Details: details,
	}

	// encode it
	e := json.NewEncoder(ctx.Writer).Encode(errorStruct)
	if e != nil {
		http.Error(ctx.Writer, e.Error(), http.StatusInternalServerError)
	}
}
