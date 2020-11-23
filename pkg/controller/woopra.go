package controller

import (
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/gin-gonic/gin"
	"net/http"
)

type woopraResponse struct {
	WoopraDomain string `json:"woopra-domain"`
}


// Woopra implements the segment endpoint, which is invoked to
// retrieve the woopra domain for the ui.
type Woopra struct {
	config configuration.Configuration
}

// NewWoopra returns a new Woopra instance.
func NewWoopra(config configuration.Configuration) *Woopra {
	return &Woopra{
		config: config,
	}
}

// GetHandler returns the woopra-domain for UI.
func (w *Woopra) GetWoopraDomain(ctx *gin.Context) {
	domain := w.config.GetWoopraDomain()
	if domain != "" {
		ctx.JSON(http.StatusOK, woopraResponse{WoopraDomain: domain})
	} else {
		ctx.Status(http.StatusNotFound)
	}
}
