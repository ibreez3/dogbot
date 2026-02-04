package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/openclaw/go-openclaw/internal/protocol"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 8192
)

// Conn represents a WebSocket connection
type Conn struct {
	conn        *websocket.Conn
	send        chan []byte
	receive     chan *protocol.ProtocolMessage
	connectedAt time.Time
	lastSeen    time.Time
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	serializer  *protocol.Serializer
}

// NewConn creates a new WebSocket connection wrapper
func NewConn(wsConn *websocket.Conn) *Conn {
	ctx, cancel := context.WithCancel(context.Background())

	return &Conn{
		conn:        wsConn,
		send:        make(chan []byte, 256),
		receive:     make(chan *protocol.ProtocolMessage, 64),
		connectedAt: time.Now(),
		lastSeen:    time.Now(),
		ctx:         ctx,
		cancel:      cancel,
		serializer:  protocol.NewSerializer(),
	}
}

// SetReadDeadline sets read deadline
func (c *Conn) SetReadDeadline(deadline time.Time) error {
	return c.conn.SetReadDeadline(deadline)
}

// SetWriteDeadline sets write deadline
func (c *Conn) SetWriteDeadline(deadline time.Time) error {
	return c.conn.SetWriteDeadline(deadline)
}

// Write writes a message to the WebSocket connection
func (c *Conn) Write(data []byte) error {
	select {
	case c.send <- data:
		return nil
	case <-c.ctx.Done():
		return fmt.Errorf("connection closed")
	default:
		return fmt.Errorf("send buffer full")
	}
}

// WriteMessage writes a protocol message
func (c *Conn) WriteMessage(msg *protocol.ProtocolMessage) error {
	data, err := c.serializer.Marshal(msg)
	if err != nil {
		return err
	}

	return c.Write(data)
}

// WriteRequest writes a request message
func (c *Conn) WriteRequest(id, method string, params interface{}) error {
	data, err := c.serializer.MarshalRequest(id, method, params)
	if err != nil {
		return err
	}

	return c.Write(data)
}

// WriteResponse writes a response message
func (c *Conn) WriteResponse(id string, ok bool, payload interface{}, errorMsg string) error {
	data, err := c.serializer.MarshalResponse(id, ok, payload, errorMsg)
	if err != nil {
		return err
	}

	return c.Write(data)
}

// WriteEvent writes an event message
func (c *Conn) WriteEvent(event string, data interface{}, seq int) error {
	serialized, err := c.serializer.MarshalEvent(event, data, seq)
	if err != nil {
		return err
	}

	return c.Write(serialized)
}

// WriteRaw writes raw data to the WebSocket connection
func (c *Conn) WriteRaw(data []byte) error {
	select {
	case c.send <- data:
		return nil
	case <-c.ctx.Done():
		return fmt.Errorf("connection closed")
	default:
		return fmt.Errorf("send buffer full")
	}
}

// Receive returns a channel for incoming messages
func (c *Conn) Receive() <-chan *protocol.ProtocolMessage {
	return c.receive
}

// Start starts the connection's read/write pumps
func (c *Conn) Start() {
	go c.readPump()
	go c.writePump()
}

// Stop stops the connection
func (c *Conn) Stop() {
	c.cancel()
	c.conn.Close()
}

// Close closes the connection
func (c *Conn) Close() error {
	c.cancel()
	return c.conn.Close()
}

// ID returns a unique ID for this connection
func (c *Conn) ID() string {
	return fmt.Sprintf("conn-%d", c.connectedAt.UnixNano())
}

// ConnectedAt returns the connection time
func (c *Conn) ConnectedAt() time.Time {
	return c.connectedAt
}

// LastSeen returns the last activity time
func (c *Conn) LastSeen() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastSeen
}

// UpdateLastSeen updates the last activity time
func (c *Conn) UpdateLastSeen() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastSeen = time.Now()
}

// IsAlive checks if the connection is alive
func (c *Conn) IsAlive() bool {
	select {
	case <-c.ctx.Done():
		return false
	default:
		return true
	}
}

// readPump pumps messages from the WebSocket connection to the receive channel
func (c *Conn) readPump() {
	defer func() {
		close(c.receive)
		c.cancel()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.SetReadDeadline(time.Now().Add(pongWait))

	// Setup pong handler to reset read deadline
	c.conn.SetPongHandler(func(string) error {
		c.SetReadDeadline(time.Now().Add(pongWait))
		c.UpdateLastSeen()
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}

		// Unmarshal message
		msg, err := c.serializer.Unmarshal(message)
		if err != nil {
			log.Printf("Message unmarshal error: %v", err)
			continue
		}

		c.UpdateLastSeen()

		// Send to receive channel
		select {
		case c.receive <- msg:
		case <-c.ctx.Done():
			return
		default:
			log.Printf("Receive buffer full, dropping message")
		}
	}
}

// writePump pumps messages from the send channel to the WebSocket connection
func (c *Conn) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
		c.cancel()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Channel closed
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Write message
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			// Send ping
			c.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-c.ctx.Done():
			return
		}
	}
}

// MessageHandler is a function that handles incoming messages
type MessageHandler func(conn *Conn, msg *protocol.ProtocolMessage) error

// Handler manages WebSocket message handling
type Handler struct {
	serializer *protocol.Serializer
	handlers   map[string]MessageHandler
	mu         sync.RWMutex
}

// NewHandler creates a new message handler
func NewHandler() *Handler {
	return &Handler{
		serializer: protocol.NewSerializer(),
		handlers:   make(map[string]MessageHandler),
	}
}

// RegisterHandler registers a message handler for a specific method
func (h *Handler) RegisterHandler(method string, handler MessageHandler) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.handlers[method] = handler
}

// HandleMessage handles an incoming message
func (h *Handler) HandleMessage(conn *Conn, msg *protocol.ProtocolMessage) error {
	if msg.Type == protocol.TypeReq && msg.Method != "" {
		return h.handleRequest(conn, msg)
	}

	// Default: log and ignore
	log.Printf("Unhandled message: type=%s method=%s id=%s", msg.Type, msg.Method, msg.ID)
	return nil
}

// handleRequest handles a request message
func (h *Handler) handleRequest(conn *Conn, msg *protocol.ProtocolMessage) error {
	h.mu.RLock()
	handler, ok := h.handlers[msg.Method]
	h.mu.RUnlock()

	if !ok {
		return conn.WriteResponse(msg.ID, false, nil, fmt.Sprintf("unknown method: %s", msg.Method))
	}

	if err := handler(conn, msg); err != nil {
		return conn.WriteResponse(msg.ID, false, nil, err.Error())
	}

	return nil
}

// DefaultHandler creates a default handler with common handlers registered
func DefaultHandler() *Handler {
	h := NewHandler()

	// Register connect handler
	h.RegisterHandler("connect", func(conn *Conn, msg *protocol.ProtocolMessage) error {
		var req protocol.ConnectRequest
		if err := json.Unmarshal(msg.Params, &req); err != nil {
			return err
		}

		// Validate request
		if err := req.Validate(); err != nil {
			return err
		}

		// Send hello response
		sessionID := fmt.Sprintf("session-%d", time.Now().UnixNano())
		response := protocol.HelloResponse{
			Type: string(protocol.TypeRes),
			ID:   msg.ID,
			Ok:   true,
			Payload: protocol.HelloPayload{
				Version:   "0.0.1",
				DeviceID:  req.DeviceID,
				SessionID: sessionID,
				Workspace: "default",
				State: &protocol.StateSnapshot{
					Version:   "0.0.1",
					SessionID: sessionID,
					Workspace: "default",
					Timestamp: time.Now().Unix(),
				},
			},
		}

		data, _ := json.Marshal(response)
		return conn.Write(data)
	})

	// Register ping handler
	h.RegisterHandler("ping", func(conn *Conn, msg *protocol.ProtocolMessage) error {
		var ping protocol.PingMessage
		if err := json.Unmarshal(msg.Params, &ping); err != nil {
			return err
		}

		// Send pong response
		return conn.WriteResponse(msg.ID, true, protocol.PingMessage{Seq: ping.Seq}, "")
	})

	// Register state handler
	h.RegisterHandler("state", func(conn *Conn, msg *protocol.ProtocolMessage) error {
		var req protocol.StateRequest
		if err := json.Unmarshal(msg.Params, &req); err != nil {
			return err
		}

		// Return current state
		state := &protocol.StateSnapshot{
			Version:   "0.0.1",
			SessionID: conn.ID(),
			Timestamp: time.Now().Unix(),
		}

		return conn.WriteResponse(msg.ID, true, state, "")
	})

	return h
}
