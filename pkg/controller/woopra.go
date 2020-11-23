package controller

import (
	"github.com/codeready-toolchain/registration-service/pkg/application"
	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/gin-gonic/gin"
)

// Woopra implements the segment endpoint, which is invoked to
// retrieve the woopra domain for the ui.
type Woopra struct {
	app    application.Application
	config configuration.Configuration
}

// NewWoopra returns a new Woopra instance.
func NewWoopra(app application.Application, config configuration.Configuration) *Woopra {
	return &Woopra{
		config: config,
		app: app,
	}
}

// GetHandler returns the woopra-domain for UI.
func (w *Woopra) GetWoopraDomain(ctx *gin.Context) {
	w.app.WoopraService().GetWoopraDomain(ctx)
}
