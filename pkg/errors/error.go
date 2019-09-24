package errors

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Error struct {
	Status      string
	Code        int
	Message     string
	Description string
}

func EncodeError(ctx *gin.Context, err error, code int, description string) {
	// construct an error struct out of err and ctx
	errorStruct := &Error{
		Status:      http.StatusText(code),
		Code:        code,
		Message:     err.Error(),
		Description: description,
	}

	// encode it
	e := json.NewEncoder(ctx.Writer).Encode(errorStruct)
	if e != nil {
		http.Error(ctx.Writer, e.Error(), http.StatusInternalServerError)
	}
}
