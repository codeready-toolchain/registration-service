package controller

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/gin-gonic/gin"
)

// SignupCheck implements the SignupCheck endpoint.
type SignupCheck struct {
	config *configuration.Registry
	logger *log.Logger
	testRequestTimestamp int64
}

// SignupCheckPayload payload
type SignupCheckPayload struct {
	ProvisioningDone bool `json:"provisioning_done"`
}

// NewSignupCheck returns a new SignupCheck instance.
func NewSignupCheck(logger *log.Logger, config *configuration.Registry) *SignupCheck {
	return &SignupCheck{
		logger: logger,
		config: config,
	}
}

// getSignupCheckInfo returns the SignupCheck info.
func (hc *SignupCheck) getSignupCheckInfo() *SignupCheckPayload {
	return &SignupCheckPayload{
		ProvisioningDone: true,
	}
}

// GetHandler returns a default heath check result.
func (hc *SignupCheck) GetHandler(ctx *gin.Context) {
	// Default handler for system SignupCheck
	ctx.Writer.Header().Set("Content-Type", "application/json")
	SignupCheckInfo := hc.getSignupCheckInfo()
	if hc.config.IsTestingMode() {
		// testing mode, wait 5s from the first request to flag state complete.
		if hc.testRequestTimestamp == 0 {
			hc.testRequestTimestamp = time.Now().Unix()
		}
		if time.Now().Unix() - hc.testRequestTimestamp >= 5 {
			SignupCheckInfo.ProvisioningDone = true
		} else {
			SignupCheckInfo.ProvisioningDone = false
		}
	}
	ctx.Writer.WriteHeader(http.StatusOK)
	err := json.NewEncoder(ctx.Writer).Encode(SignupCheckInfo)
	if err != nil {
		hc.logger.Println("error writing response body", err.Error())
		errors.EncodeError(ctx, err, http.StatusInternalServerError, "error writing response body")
	}
}
