package websockets

import (
	"errors"
	"log"
	"time"

	"github.com/codeready-toolchain/registration-service/pkg/middleware"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	// time allowed to write a message to the peer.
	writeWait = 10 * time.Second
	// time allowed to read the next pong message from the peer.
	pongWait = 10 * time.Second
	// send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
	// maximum message size allowed from peer.
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Message represents a message over the channel
type Message struct {
	// subscription of the peer identity
	Sub string
	// message data
	Body []byte
}

// Hub maintains the set of active clients
type Hub struct {
	// registered clients, mapped to the sub id.
	clients map[*Client]string
	// outbound messages to clients.
	Outbound chan *Message
	// inbound messages from clients.
	Inbound chan *Message
	// register requests from clients.
	register chan *Client
	// unregister requests from clients.
	unregister chan *Client
}

// NewHub creates a new Hub instance.
func NewHub() *Hub {
	log.Println("creating new websockets hub")
	hub := &Hub{
		Outbound:   make(chan *Message),
		Inbound:    make(chan *Message),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]string),
	}
	go hub.run()
	return hub
}

// Clients returns the known clients map.
func (h *Hub) Clients() map[*Client]string {
	return h.clients
}

// Run runs the hub's main loop.
func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			log.Printf("hub: registering client for sub %s", client.sub)
			h.clients[client] = client.sub
			log.Printf("hub: done registering client for sub %s (current len %d)", client.sub, len(h.clients))
		case client := <-h.unregister:
			log.Printf("hub: trying to unregister client for sub %s", client.sub)
			if _, ok := h.clients[client]; ok {
				log.Printf("hub: unregistering client for sub %s", client.sub)
				delete(h.clients, client)
				close(client.send)
			} else {
				log.Printf("hub: client for sub %s not found while unregistering", client.sub)
			}
		case message := <-h.Outbound:
			// message appeared on the outbound channel, find the matching client
			log.Printf("hub: detected outbound message for sub %s", message.Sub)
			for client, sub := range h.clients {
				if sub == message.Sub {
					// found the client, send message out
					log.Printf("hub: found client for sub %s", message.Sub)
					select {
					case client.send <- message.Body:
						log.Printf("hub: successfully sent outbound message to sub %s", message.Sub)
					default:
						// send message failed, terminate conn with this client
						log.Printf("hub: error sending outbound message to sub %s", message.Sub)
						close(client.send)
						delete(h.clients, client)
					}
				}
			}
		}
	}
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub *Hub
	// the websocket connection.
	conn *websocket.Conn
	// the sub of the client identity
	sub string
	// buffered channel of outbound messages.
	send chan []byte
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	defer func() {
		log.Printf("readPump unregistering connection with sub %s", c.sub)
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(s string) error {
		// received pong, reset deadline for this conn
		err := c.conn.SetReadDeadline(time.Now().Add(pongWait))
		if err != nil {
			log.Printf("error setting read deadline on websocket connection for sub %s: %s", c.sub, err.Error())
		}
		return nil
	})
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("error reading from websocket connection with sub %s: %v", c.sub, err)
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error closing websocket connection with sub %s: %v", c.sub, err)
			}
			break
		}
		receivedMessage := &Message{
			Sub:  c.sub,
			Body: message,
		}
		log.Printf("connection received message from sub %s: %s", receivedMessage.Sub, receivedMessage.Body)
		// put the message on the hub channel
		c.hub.Inbound <- receivedMessage
		log.Printf("received message from sub %s successfully committed to inbound channel", receivedMessage.Sub)
	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			// a message is on the outbound client queue
			err := c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err != nil {
				log.Printf("error setting write deadline on websocket connection for sub %s: %s", c.sub, err.Error())
				return
			}
			if !ok {
				log.Printf("hub closed channel on websocket connection for sub %s", c.sub)
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Printf("error getting writer on websocket connection for sub %s: %s", c.sub, err.Error())
				return
			}
			_, err = w.Write(message)
			if err != nil {
				log.Printf("error writing to websocket connection for sub %s: %s", c.sub, err.Error())
				return
			}
			// Add queued chat messages to the current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				_, err = w.Write(<-c.send)
				if err != nil {
					log.Printf("error writing to websocket connection for sub %s: %s", c.sub, err.Error())
					return
				}
			}
			if err := w.Close(); err != nil {
				log.Printf("error closing writer on websocket connection for sub %s: %s", c.sub, err.Error())
				return
			}
		case <-ticker.C:
			// the ticker kicked in, refresh the deadline for this connection
			err := c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err != nil {
				log.Printf("error setting write deadline on websocket connection for sub %s: %s", c.sub, err.Error())
			}
			if err = c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("error sending ping on websocket connection for sub %s: %s", c.sub, err.Error())
				return
			}
		}
	}
}

// HTTPHandler handles websocket requests from the peers.
func HTTPHandler(hubInstance *Hub, c *gin.Context) {
	w := c.Writer
	r := c.Request
	// the subject is injected into the context by the
	// auth middleware and is trusted info
	subject, exists := c.Get(middleware.SubKey)
	if !exists {
		log.Println(errors.New("websocket connect without subject claim"))
		return
	}
	subjStr, ok := subject.(string)
	if !ok {
		log.Println(errors.New("websocket connect with non-string subject claim"))
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("error upgrading socket connection for websocket communication for sub %s: %s", subjStr, err.Error())
		log.Println(err)
		return
	}
	client := &Client{hub: hubInstance, conn: conn, sub: subjStr, send: make(chan []byte, 256)}
	log.Printf("registering client sub %s", subjStr)
	hubInstance.register <- client
	// launch goroutines for read and write to the connection
	go client.writePump()
	go client.readPump()
}
