package controller

import (
	"github.com/gin-gonic/gin"
	"net/http"
)


// PreFlight implements the preflight options endpoint.
type PreFlight struct {
}

// NewPreFlight returns a new PreFlight instance.
func NewPreFlight() *PreFlight {
	return &PreFlight{}
}

// PreflightHandler is to check if the next request is allowed to go out of the domain.
func (pf PreFlight) PreflightHandler(ctx *gin.Context) {
	ctx.Header("Access-Control-Allow-Origin", "*")
	ctx.Header("Access-Control-Allow-Headers", "access-control-allow-origin, access-control-allow-headers")
	ctx.JSON(http.StatusOK, struct{}{})
}