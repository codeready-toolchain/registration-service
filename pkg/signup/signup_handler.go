package signup

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"

	"github.com/gin-gonic/gin"
)

// SignupService implements the signup endpoint, which is invoked for new user registrations.
type SignupService struct {
	config *configuration.Registry
	logger *log.Logger
	keyManager *KeyManager
	tokenParser *TokenParser
}

// NewSignupService returns a new SignupService instance.
func NewSignupService(logger *log.Logger, config *configuration.Registry) (*SignupService, error) {
	service := &SignupService{
		logger: logger,
		config: config,
	}
	// create new KeyManager
	keyManager, err := NewKeyManager(logger, config)
	service.keyManager = keyManager
	if err != nil {
		return nil, err
	}
	// create new TokenParser
	tokenParser, err := NewTokenParser(logger, config, keyManager)
	if err != nil {
		return nil, err
	}
	service.tokenParser = tokenParser
	// return new service
	return service, nil
}

// PostSignupHandler returns signup info.
func (srv *SignupService) PostSignupHandler(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", "application/json")
	// get JWT encoded string from request
	jwt := ctx.PostForm("jwt")
	if jwt == "" {
		http.Error(ctx.Writer, "jwt field empty", http.StatusInternalServerError)
		return
	}
	// parse JWT
	claims, err := srv.tokenParser.FromString(jwt)
	if err != nil {
		http.Error(ctx.Writer, err.Error(), http.StatusBadRequest)
		return
	}
	// TODO: use claims to validate username and create CRD
	srv.logger.Printf("JWT parsed: username==%s email==%s", claims.Username, claims.Email)
	// send response
	err = json.NewEncoder(ctx.Writer).Encode(nil)
	if err != nil {
		http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
		return
	}
}
