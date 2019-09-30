package controller

import (
	"log"

	"github.com/codeready-toolchain/registration-service/pkg/configuration"
	"github.com/codeready-toolchain/registration-service/pkg/websockets"
)

// WebsocketsHandler implements the websockets handler controller.
type WebsocketsHandler struct {
	config *configuration.Registry
	logger *log.Logger
	hub    *websockets.Hub
}

// NewWebsocketsHandler returns a new WebsocketsHandler instance.
func NewWebsocketsHandler(logger *log.Logger, config *configuration.Registry, websocketsHub *websockets.Hub) *WebsocketsHandler {
	return &WebsocketsHandler{
		logger: logger,
		config: config,
		hub: websocketsHub,
	}
}

// Message handles an incoming message from websockets.
func (ws *WebsocketsHandler) Message(subject string, message []byte) {	
	if ws.config.IsTestingMode() {
		log.Printf("Message Handler received socket message from %s: %s", subject, message)
		// testmode, reply to echotest
		clientAvailable := ws.hub.SendMessage(subject, append([]byte(subject + " %RESPONSE% "), message...))
		if !clientAvailable {
			log.Printf("error, client not connected for subject %s", subject)
		}
	}
}
