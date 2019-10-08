package controller

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/errors"
	"github.com/gin-gonic/gin"
)

const (
	// SignupStatePending represents a signup in state pending approval.
	SignupStatePending = "pendingApproval"
	// SignupStateProvisioning represents a signup in state in provisioning.
	SignupStateProvisioning = "provisioning"
	// SignupStateProvisioned represents a signup in state provisioned/ready.
	SignupStateProvisioned = "provisioned"
)

// Signup implements the signup endpoint, which is invoked for new user registrations.
type Signup struct {
	config *configuration.Registry
	logger *log.Logger
	checkerFunc func(ctx *gin.Context) *SignupCheckPayload
}

// SignupCheckPayload payload
type SignupCheckPayload struct {
	Ready   bool   `json:"ready"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// NewSignup returns a new Signup instance.
func NewSignup(logger *log.Logger, config *configuration.Registry, checker func(ctx *gin.Context) *SignupCheckPayload) *Signup {
	sc := &Signup {
		logger: logger,
		config: config,
	}
	if checker != nil {
		sc.checkerFunc = checker
	} else {
		sc.checkerFunc = sc.getSignupCheckInfo
	}
	return sc
}

// getSignupCheckInfo returns the SignupCheck info.
func (hc *Signup) getSignupCheckInfo(ctx *gin.Context) *SignupCheckPayload {
	// the integration with the actual k8s api needs to retrieve the
	// user details from the context here (added by the middleware) and
	// check the provisioning state.
	return &SignupCheckPayload{
		Ready:   true,
		Reason:  "",
		Message: "",
	}
}

// PostHandler starts the signup process.
func (s *Signup) PostHandler(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", "application/json")

	// the KeyManager can be accessed here: auth.DefaultKeyManager()

	err := json.NewEncoder(ctx.Writer).Encode(nil)
	if err != nil {
		s.logger.Println("error writing response body", err.Error())
		errors.EncodeError(ctx, err, http.StatusInternalServerError, "error writing response body")
	}
}

// GetHandler returns the signup check result.
func (s *Signup) GetHandler(ctx *gin.Context) {
	// Default handler for system SignupCheck
	ctx.Writer.Header().Set("Content-Type", "application/json")
	signupCheckInfo := s.checkerFunc(ctx)
	ctx.Writer.WriteHeader(http.StatusOK)
	err := json.NewEncoder(ctx.Writer).Encode(signupCheckInfo)
	if err != nil {
		s.logger.Println("error writing response body", err.Error())
		errors.EncodeError(ctx, err, http.StatusInternalServerError, "error writing response body")
	}
}

