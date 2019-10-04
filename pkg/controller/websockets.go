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
func NewWebsocketsHandler(logger *log.Logger, config *configuration.Registry) *WebsocketsHandler {
	h := &WebsocketsHandler{
		logger: logger,
		config: config,
		hub: websockets.NewHub(),
	}
	go h.messageHandler()
	return h  
}

// Outbound returns the outbound message channel that can be used by 
// other controllers to send messages.
func (ws *WebsocketsHandler) Outbound() chan *websockets.Message {
	return ws.hub.Outbound
}

// Hub provides access to the underlying websockets hub.
func (ws *WebsocketsHandler) Hub() *websockets.Hub {
	return ws.hub
}

// Message handles an incoming message from websockets.
func (ws *WebsocketsHandler) messageHandler() {	

	for message := range ws.Hub().Inbound {
		log.Printf("Message Handler received socket message from %s: %s", message.Sub, message.Body)
		// when in testingmode, reply to each message with a ping
		if ws.config.IsTestingMode() {
			response := `{ "sub": "` + message.Sub + `", "body": "` + string(message.Body) + `" }`
			ws.hub.Outbound <- &websockets.Message{
				Sub: message.Sub,
				Body: []byte(response),
			}
		}
	}
}
