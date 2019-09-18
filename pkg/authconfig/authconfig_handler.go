package authconfig

import (
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

// AuthconfigHandler returns signup info.
func (srv *Service) AuthconfigHandler(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", "application/json")
	ctx.Writer.WriteHeader(http.StatusOK)
	_, err := ctx.Writer.WriteString(srv.config.GetAuthClientConfigAuthRaw())
	if err != nil {
		srv.logger.Println("error writing response body", err.Error())
		http.Error(ctx.Writer, err.Error(), http.StatusInternalServerError)
		return
	}	
}
