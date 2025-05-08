package integration

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketMessage represents a message sent over websocket
type WebSocketMessage struct {
	Type      string                 `json:"type"`
	Timestamp int64                  `json:"timestamp"`
	PlaneID   string                 `json:"plane_id,omitempty"`
	Data      map[string]interface{} `json:"data"`
}

// WebSocketClient represents a connected websocket client
type WebSocketClient struct {
	ID         string
	Connection *websocket.Conn
	Send       chan []byte
	Server     *WebSocketServer
}

// WebSocketServer manages websocket connections
type WebSocketServer struct {
	clients    map[string]*WebSocketClient
	register   chan *WebSocketClient
	unregister chan *WebSocketClient
	broadcast  chan []byte
	upgrader   websocket.Upgrader
	pubsub     *PubSub
	mu         sync.RWMutex
}

// NewWebSocketServer creates a new websocket server
func NewWebSocketServer(pubsub *PubSub) *WebSocketServer {
	return &WebSocketServer{
		clients:    make(map[string]*WebSocketClient),
		register:   make(chan *WebSocketClient),
		unregister: make(chan *WebSocketClient),
		broadcast:  make(chan []byte),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all connections - should be restricted in production
			}

// Run starts the websocket server
func (server *WebSocketServer) Run() {
	for {
		select {
		case client := <-server.register:
			server.mu.Lock()
			server.clients[client.ID] = client
			server.mu.Unlock()
			log.Printf("Client connected: %s", client.ID)

		case client := <-server.unregister:
			server.mu.Lock()
			if _, ok := server.clients[client.ID]; ok {
				delete(server.clients, client.ID)
				close(client.Send)
				log.Printf("Client disconnected: %s", client.ID)
			}
			server.mu.Unlock()

		case message := <-server.broadcast:
			server.mu.RLock()
			for _, client := range server.clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(server.clients, client.ID)
				}
			}
			server.mu.RUnlock()
		}
	}
}

// HandleWebSocket handles websocket requests from clients
func (server *WebSocketServer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := server.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	clientID := fmt.Sprintf("%s_%d", conn.RemoteAddr().String(), time.Now().UnixNano())
	client := &WebSocketClient{
		ID:         clientID,
		Connection: conn,
		Send:       make(chan []byte, 256),
		Server:     server,
	}

	// Register the client
	server.register <- client

	// Start goroutines for reading and writing
	go client.readPump()
	go client.writePump()
}

// BroadcastEvent sends an event to all connected clients
func (server *WebSocketServer) BroadcastEvent(event Event) {
	message, err := SerializeEvent(event)
	if err != nil {
		log.Printf("Error serializing event: %v", err)
		return
	}

	server.broadcast <- message
}

// readPump pumps messages from the websocket connection to the hub
func (client *WebSocketClient) readPump() {
	defer func() {
		client.Server.unregister <- client
		client.Connection.Close()
	}()

	client.Connection.SetReadLimit(512 * 1024) // 512KB max message size
	client.Connection.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.Connection.SetPongHandler(func(string) error {
		client.Connection.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := client.Connection.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		// Process incoming messages (commands from client)
		var wsMessage WebSocketMessage
		if err := json.Unmarshal(message, &wsMessage); err != nil {
			log.Printf("Error parsing message: %v", err)
			continue
		}

		// Process commands
		client.handleCommand(wsMessage)
	}
}

// writePump pumps messages from the hub to the websocket connection
func (client *WebSocketClient) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		client.Connection.Close()
	}()

	for {
		select {
		case message, ok := <-client.Send:
			client.Connection.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// The hub closed the channel
				client.Connection.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := client.Connection.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current websocket message
			n := len(client.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-client.Send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			client.Connection.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.Connection.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleCommand processes commands from clients
func (client *WebSocketClient) handleCommand(message WebSocketMessage) {
	switch message.Type {
	case "subscribe":
		// Extract event type and plane ID from subscription request
		eventTypeStr, ok := message.Data["event_type"].(string)
		if !ok {
			log.Printf("Invalid subscription request: missing event_type")
			return
		}
		
		planeID, ok := message.Data["plane_id"].(string)
		if !ok {
			planeID = "" // Empty string means all planes
		}

		// Subscribe to the specified event type
		subscription, err := client.Server.pubsub.Subscribe(EventType(eventTypeStr), planeID)
		if err != nil {
			log.Printf("Subscription error: %v", err)
			return
		}

		// Forward events from this subscription to the client
		go func() {
			for event := range subscription.Channel {
				eventJSON, err := json.Marshal(event)
				if err != nil {
					log.Printf("Error serializing event: %v", err)
					continue
				}

				client.Send <- eventJSON
			}
		}()

		// Send confirmation
		response := WebSocketMessage{
			Type:      "subscription_confirm",
			Timestamp: time.Now().Unix(),
			Data: map[string]interface{}{
				"event_type": eventTypeStr,
				"plane_id":   planeID,
				"sub_id":     subscription.ID,
			},
		}
		
		responseJSON, _ := json.Marshal(response)
		client.Send <- responseJSON

	case "unsubscribe":
		// TODO: Implement unsubscribe functionality

	case "query":
		// TODO: Implement query functionality for historical data

	default:
		log.Printf("Unknown command: %s", message.Type)
	}
}

// StartWebSocketServer starts the websocket server on the specified port
func StartWebSocketServer(pubsub *PubSub, port int) error {
	server := NewWebSocketServer(pubsub)
	go server.Run()

	http.HandleFunc("/ws", server.HandleWebSocket)
	
	addr := fmt.Sprintf(":%d", port)
	log.Printf("Starting WebSocket server on %s", addr)
	
	return http.ListenAndServe(addr, nil)
},
		pubsub: pubsub,
	}
		},