// blockplain/pubsub/publisher.go
package pubsub

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/blockplain/config"
	"github.com/nats-io/nats.go"
	"github.com/gorilla/websocket"
)

// Publisher is the interface for event publishing
type Publisher interface {
	Publish(topic string, data interface{}) error
	Close() error
}

// NATSPublisher implements Publisher using NATS
type NATSPublisher struct {
	conn    *nats.Conn
	options config.PubSubConfig
}

// WebSocketPublisher implements Publisher using WebSockets
type WebSocketPublisher struct {
	clients      map[*websocket.Conn]bool
	clientsMutex sync.Mutex
	server       *websocket.Upgrader
	listener     net.Listener
	options      config.PubSubConfig
}

// SocketIOPublisher implements Publisher using Socket.IO
type SocketIOPublisher struct {
	server  *net.Listener
	clients map[string]net.Conn
	mutex   sync.RWMutex
	options config.PubSubConfig
}

// NewPublisher creates a new publisher based on config
func NewPublisher(cfg config.PubSubConfig) (Publisher, error) {
	switch cfg.Type {
	case "nats":
		return newNATSPublisher(cfg)
	case "websocket":
		return newWebSocketPublisher(cfg)
	case "socketio":
		return newSocketIOPublisher(cfg)
	default:
		return nil, fmt.Errorf("unsupported publisher type: %s", cfg.Type)
	}
}

// newNATSPublisher creates a new NATS publisher
func newNATSPublisher(cfg config.PubSubConfig) (*NATSPublisher, error) {
	opts := nats.Options{
		Url:            cfg.URL,
		AllowReconnect: true,
		MaxReconnect:   10,
		ReconnectWait:  5 * time.Second,
		Timeout:        3 * time.Second,
	}

	nc, err := opts.Connect()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %v", err)
	}

	return &NATSPublisher{
		conn:    nc,
		options: cfg,
	}, nil
}

// Publish publishes data to a NATS topic
func (p *NATSPublisher) Publish(topic string, data interface{}) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %v", err)
	}

	// Publish to NATS
	return p.conn.Publish(topic, payload)
}

// Close closes the NATS connection
func (p *NATSPublisher) Close() error {
	p.conn.Close()
	return nil
}

// newWebSocketPublisher creates a new WebSocket publisher
func newWebSocketPublisher(cfg config.PubSubConfig) (*WebSocketPublisher, error) {
	upgrader := &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins
		},
	}

	// Start WebSocket server
	listener, err := net.Listen("tcp", cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to start WebSocket server: %v", err)
	}

	publisher := &WebSocketPublisher{
		clients:      make(map[*websocket.Conn]bool),
		clientsMutex: sync.Mutex{},
		server:       upgrader,
		listener:     listener,
		options:      cfg,
	}

	// Start accepting connections
	go publisher.acceptConnections()

	return publisher, nil
}

// acceptConnections accepts WebSocket connections
func (p *WebSocketPublisher) acceptConnections() {
	httpServer := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := p.server.Upgrade(w, r, nil)
			if err != nil {
				log.Printf("Failed to upgrade connection: %v", err)
				return
			}

			// Register client
			p.clientsMutex.Lock()
			p.clients[conn] = true
			p.clientsMutex.Unlock()

			// Handle disconnection
			go func() {
				for {
					_, _, err := conn.ReadMessage()
					if err != nil {
						p.clientsMutex.Lock()
						delete(p.clients, conn)
						p.clientsMutex.Unlock()
						conn.Close()
						break
					}
				}
			}()
		}),
	}

	if err := httpServer.Serve(p.listener); err != nil {
		log.Printf("WebSocket server error: %v", err)
	}
}

// Publish publishes data to all WebSocket clients
func (p *WebSocketPublisher) Publish(topic string, data interface{}) error {
	envelope := map[string]interface{}{
		"topic": topic,
		"data":  data,
	}

	payload, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %v", err)
	}

	p.clientsMutex.Lock()
	defer p.clientsMutex.Unlock()

	for client := range p.clients {
		err := client.WriteMessage(websocket.TextMessage, payload)
		if err != nil {
			// Remove client on error
			delete(p.clients, client)
			client.Close()
		}
	}

	return nil
}

// Close closes the WebSocket server
func (p *WebSocketPublisher) Close() error {
	p.clientsMutex.Lock()
	defer p.clientsMutex.Unlock()

	// Close all client connections
	for client := range p.clients {
		client.Close()
		delete(p.clients, client)
	}

	// Close listener
	return p.listener.Close()
}

// newSocketIOPublisher creates a new Socket.IO publisher
func newSocketIOPublisher(cfg config.PubSubConfig) (*SocketIOPublisher, error) {
	listener, err := net.Listen("tcp", cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to start Socket.IO server: %v", err)
	}

	publisher := &SocketIOPublisher{
		server:  &listener,
		clients: make(map[string]net.Conn),
		mutex:   sync.RWMutex{},
		options: cfg,
	}

	// Start accepting connections
	go publisher.acceptConnections()

	return publisher, nil
}

// acceptConnections accepts Socket.IO connections
func (p *SocketIOPublisher) acceptConnections() {
	for {
		conn, err := (*p.server).Accept()
		if err != nil {
			log.Printf("Socket.IO accept error: %v", err)
			continue
		}

		clientID := fmt.Sprintf("%s-%d", conn.RemoteAddr().String(), time.Now().UnixNano())

		p.mutex.Lock()
		p.clients[clientID] = conn
		p.mutex.Unlock()

		// Handle client communication
		go p.handleClient(clientID, conn)
	}
}

// handleClient manages a Socket.IO client connection
func (p *SocketIOPublisher) handleClient(id string, conn net.Conn) {
	defer func() {
		conn.Close()
		p.mutex.Lock()
		delete(p.clients, id)
		p.mutex.Unlock()
	}()

	buffer := make([]byte, 1024)
	for {
		_, err := conn.Read(buffer)
		if err != nil {
			// Client disconnected
			break
		}
		// We're not actually processing messages here, just monitoring for disconnections
	}
}

// Publish publishes data to all Socket.IO clients
func (p *SocketIOPublisher) Publish(topic string, data interface{}) error {
	envelope := map[string]interface{}{
		"topic": topic,
		"data":  data,
	}

	payload, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %v", err)
	}

	// Add Socket.IO protocol framing
	frameData := fmt.Sprintf("42[\"%s\",%s]", topic, string(payload))

	p.mutex.RLock()
	defer p.mutex.RUnlock()

	// Send to all clients
	for id, client := range p.clients {
		_, err := client.Write([]byte(frameData))
		if err != nil {
			log.Printf("Failed to write to client %s: %v", id, err)
			// Client will be removed when read goroutine detects disconnection
		}
	}

	return nil
}

// Close closes the Socket.IO server
func (p *SocketIOPublisher) Close() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Close all client connections
	for id, client := range p.clients {
		client.Close()
		delete(p.clients, id)
	}

	// Close listener
	return (*p.server).Close()
}