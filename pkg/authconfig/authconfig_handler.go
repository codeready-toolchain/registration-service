package authconfig

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/gin-gonic/gin"
)

// Service implements the auth config endpoint, which is invoked to 
// retrieve the auth config for the ui.
type Service struct {
	config *configuration.Registry
	logger *log.Logger
}

// New returns a new Service instance.
func New(logger *log.Logger, config *configuration.Registry) *Service {
	return &Service{
		logger: logger,
		config: config,
	}
}

func (srv *Service) isJSON(s string) bool {
	var elements map[string]interface{}
	err := json.Unmarshal([]byte(s), &elements)
	if err!=nil {
		srv.logger.Println("error parsing auth config JSON:", err.Error())
		return false
	}
	return true
}

// AuthconfigHandler returns signup info.
func (srv *Service) AuthconfigHandler(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", "application/json")
	ctx.Writer.WriteHeader(http.StatusOK)
	configStr := srv.config.GetAuthClientConfigAuthJSON()
 	if !srv.isJSON(configStr) {
		srv.logger.Println("config JSON not valid, responding with server error.")
		http.Error(ctx.Writer, "auth client config is in wrong format (json unmarshal failed)", http.StatusInternalServerError)
		return
	}
	_, err := ctx.Writer.WriteString(srv.config.GetAuthClientConfigAuthJSON())
	if err != nil {
		srv.logger.Println("error writing response body", err.Error())
		http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
		return
	}	
}
