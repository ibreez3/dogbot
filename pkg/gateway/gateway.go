package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/fasthttp/websocket"
	agent "github.com/openclaw/go-openclaw/internal/agent"
	"github.com/openclaw/go-openclaw/internal/protocol"
	"github.com/openclaw/go-openclaw/internal/ws"
	"github.com/valyala/fasthttp"
)

var ErrServerClosed = errors.New("server closed")

// GatewayState represents the gateway state
type GatewayState struct {
	Running bool   `json:"running"`
	Version string `json:"version"`
	Stats  *GatewayStats `json:"stats,omitempty"`
}

// GatewayStats represents gateway statistics
type GatewayStats struct {
	ClientCount int `json:"client_count"`
	Uptime      int64 `json:"uptime"` // uptime in seconds
}

// Gateway represents a WebSocket gateway server
type Gateway struct {
	addr         string
	id           string
	clients      map[string]*Client // client ID -> client
	clientsLock  sync.RWMutex
	register     chan *Client
	unregister   chan *Client
	broadcast    chan []byte
	eventBus     *protocol.EventBus
	server       *fasthttp.Server
	upgrader     websocket.FastHTTPUpgrader
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	handler      *ws.Handler
	agentRuntime *agent.Runtime // NEW: Agent runtime
}

// New creates a new gateway instance
func New(addr string) *Gateway {
	ctx, cancel := context.WithCancel(context.Background())

	return &Gateway{
		addr:       addr,
		id:         fmt.Sprintf("gateway-%d", time.Now().UnixNano()),
		clients:    make(map[string]*Client),
		register:   make(chan *Client, 64),
		unregister: make(chan *Client, 64),
		broadcast:  make(chan []byte, 256),
		eventBus:   protocol.NewEventBus(),
		ctx:        ctx,
		cancel:     cancel,
		handler:    ws.DefaultHandler(),
		agentRuntime: nil, // NEW: Agent runtime placeholder
		upgrader: websocket.FastHTTPUpgrader{
			ReadBufferSize: 1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(ctx *fasthttp.RequestCtx) bool {
				return true // Allow all origins for now
			},
		},
	}
}

// Start starts of gateway server
func (g *Gateway) Start(ctx context.Context) error {
	g.ctx = ctx

	g.server = &fasthttp.Server{
		Handler: g.handleHTTP,
	}

	ln, err := net.Listen("tcp", g.addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", g.addr, err)
	}

	log.Printf("🌐 Gateway listening on %s (id=%s)", g.addr, g.id)

	// Start hub
	g.wg.Add(1)
	go g.runHub()

	// Start server in background
	go func() {
		defer g.wg.Done()
		if err := g.server.Serve(ln); err != nil && !errors.Is(err, ErrServerClosed) {
			log.Printf("Server error: %v", err)
		}
	}()

	return nil
}

// Stop stops gateway server
func (g *Gateway) Stop(ctx context.Context) error {
	log.Println("🛑 Stopping Gateway...")

	g.cancel()

	// Close server
	if g.server != nil {
		if err := g.server.Shutdown(); err != nil {
			return fmt.Errorf("failed to shutdown server: %w", err)
		}
	}

	// Close all clients
	g.clientsLock.Lock()
	for _, client := range g.clients {
		client.Close()
	}
	g.clientsLock.Unlock()

	// Stop agent runtime if running
	if g.agentRuntime != nil && g.agentRuntime.Status() == "running" {
		if err := g.agentRuntime.Stop(ctx); err != nil {
			log.Printf("Failed to stop agent runtime: %v", err)
		}
	}

	// Wait for hub
	g.wg.Wait()

	log.Println("✅ Gateway stopped")
	return nil
}

// StartAgent initializes and starts agent runtime
func (g *Gateway) StartAgent(ctx context.Context, config *agent.Config) error {
	if g.agentRuntime != nil && g.agentRuntime.Status() == "running" {
		return fmt.Errorf("agent runtime is already running")
	}

	// Create agent runtime
	runtime, err := agent.NewRuntime(config)
	if err != nil {
		return fmt.Errorf("failed to create agent runtime: %w", err)
	}

	g.agentRuntime = runtime

	// Start agent runtime
	if err := g.agentRuntime.Start(); err != nil {
		return fmt.Errorf("failed to start agent runtime: %w", err)
	}

	log.Printf("🤖 Agent runtime started (provider=%s, model=%s)",
		runtime.GetStats().LLMProvider, runtime.GetStats().LLMModel)

	return nil
}

// StopAgent stops agent runtime
func (g *Gateway) StopAgent(ctx context.Context) error {
	if g.agentRuntime == nil {
		return fmt.Errorf("agent runtime is not running")
	}

	if err := g.agentRuntime.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop agent runtime: %w", err)
	}

	g.agentRuntime = nil
	log.Printf("🛑 Agent runtime stopped")
	return nil
}

// GetAgentStatus returns current status of agent runtime
func (g *Gateway) GetAgentStatus() string {
	if g.agentRuntime == nil {
		return "not_started"
	}

	return g.agentRuntime.Status()
}

// handleHTTP handles HTTP requests
func (g *Gateway) handleHTTP(ctx *fasthttp.RequestCtx) {
	path := string(ctx.Path())

	// WebSocket upgrade for /ws path
	if path == "/ws" {
		g.handleWebSocket(ctx)
		return
	}

	// Health check
	if path == "/health" {
		ctx.Response.SetStatusCode(fasthttp.StatusOK)
		ctx.Response.SetBody([]byte(`{"status":"ok","gateway_id":"`+g.id+`"}`))
		return
	}

	// Agent status endpoint
	if path == "/agent/status" {
		g.handleAgentStatusHTTP(ctx)
		return
	}

	// Not found
	ctx.Response.SetStatusCode(fasthttp.StatusNotFound)
}

// handleAgentStatusHTTP handles agent status HTTP requests
func (g *Gateway) handleAgentStatusHTTP(ctx *fasthttp.RequestCtx) {
	status := "not_started"
	if g.agentRuntime != nil {
		status = g.agentRuntime.Status()
	}

	ctx.Response.SetStatusCode(fasthttp.StatusOK)
	body := map[string]interface{}{
		"status":     status,
		"gateway_id": g.id,
	}

	if g.agentRuntime != nil && g.agentRuntime.Status() == "running" {
		body["runtime"] = g.agentRuntime.GetStats()
	}

	jsonBody, _ := json.Marshal(body)
	ctx.Response.SetBody(jsonBody)
}

// handleWebSocket handles WebSocket connections
func (g *Gateway) handleWebSocket(ctx *fasthttp.RequestCtx) {
	if err := g.upgrader.Upgrade(ctx, g.handleConnection); err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
	}
}

// handleConnection handles a WebSocket connection
func (g *Gateway) handleConnection(wsConn *websocket.Conn) {
	// Wrap WebSocket connection
	conn := ws.NewConn(wsConn)
	connID := conn.ID()

	// Create client
	client := &Client{
		ID:         connID,
		Conn:       conn,
		gateway:    g,
		connectedAt: time.Now(),
		lastSeen:   time.Now(),
		status:     "connected",
		metadata:   make(map[string]string),
	}

	// Register client
	g.register <- client

	// Start connection pumps
	conn.Start()

	// Handle messages
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		g.handleClientMessages(client)
	}()

	log.Printf("📱 Client connected: %s", connID)
}

// handleClientMessages handles incoming messages from a client
func (g *Gateway) handleClientMessages(client *Client) {
	for {
		select {
		case msg, ok := <-client.Conn.Receive():
			if !ok {
				return
			}

			// Handle message
			if err := g.handleMessage(client, msg); err != nil {
				log.Printf("Message handling error for %s: %v", client.ID, err)
			}

		case <-g.ctx.Done():
			return
		}
	}
}

// handleMessage handles an incoming message from a client
func (g *Gateway) handleMessage(client *Client, msg *protocol.ProtocolMessage) error {
	client.lastSeen = time.Now()

	// Handle agent commands
	if msg.Type == protocol.TypeReq && msg.Method == "agent.start" {
		return g.handleAgentStart(client, msg)
	}
	if msg.Type == protocol.TypeReq && msg.Method == "agent.stop" {
		return g.handleAgentStop(client, msg)
	}
	if msg.Type == protocol.TypeReq && msg.Method == "agent.status" {
		return g.handleAgentStatus(client, msg)
	}

	// Handle connect message
	if msg.Type == protocol.TypeReq && msg.Method == "connect" {
		return g.handleConnect(client, msg)
	}

	// Use protocol handler
	if err := g.handler.HandleMessage(client.Conn, msg); err != nil {
		log.Printf("Handler error: %v", err)
		return err
	}

	// Log message
	log.Printf("📨 %s: type=%s method=%s id=%s", client.ID, msg.Type, msg.Method, msg.ID)
	return nil
}

// handleAgentStart starts of agent runtime
func (g *Gateway) handleAgentStart(client *Client, msg *protocol.ProtocolMessage) error {
	// Extract config from params
	var config agent.Config
	if err := json.Unmarshal(msg.Params, &config); err != nil {
		return err
	}

	// Start agent
	if err := g.StartAgent(context.Background(), &config); err != nil {
		return err
	}

	// Send success response
	return client.Conn.WriteResponse(msg.ID, true, map[string]interface{}{
		"status": "started",
		"runtime": g.agentRuntime.GetStats(),
	}, "")
}

// handleAgentStop stops agent runtime
func (g *Gateway) handleAgentStop(client *Client, msg *protocol.ProtocolMessage) error {
	// Stop agent
	if err := g.StopAgent(context.Background()); err != nil {
		return err
	}

	// Send success response
	return client.Conn.WriteResponse(msg.ID, true, map[string]interface{}{
		"status": "stopped",
	}, "")
}

// handleAgentStatus returns agent runtime status
func (g *Gateway) handleAgentStatus(client *Client, msg *protocol.ProtocolMessage) error {
	status := g.GetAgentStatus()
	stats := map[string]interface{}{"status": status}

	if g.agentRuntime != nil && g.agentRuntime.Status() == "running" {
		stats["runtime"] = g.agentRuntime.GetStats()
	}

	return client.Conn.WriteResponse(msg.ID, true, stats, "")
}

// handleConnect handles connect handshake
func (g *Gateway) handleConnect(client *Client, msg *protocol.ProtocolMessage) error {
	var req protocol.ConnectRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return err
	}

	// Validate request
	if err := req.Validate(); err != nil {
		return err
	}

	// Update client info
	client.deviceID = req.DeviceID
	client.clientID = req.ClientID
	client.metadata["version"] = req.Version

	// Generate session ID
	sessionID := fmt.Sprintf("session-%d", time.Now().UnixNano())
	client.sessionID = sessionID

	// Create state snapshot
	state := &protocol.StateSnapshot{
		Version:   "0.0.1",
		GatewayID:  g.id,
		ClientID:   client.ID,
		SessionID:  sessionID,
		Workspace:  "default",
		Timestamp:  time.Now().Unix(),
		Metadata:   map[string]string{"gateway": g.id},
	}

	// Send hello response
	response := protocol.HelloResponse{
		Type: string(protocol.TypeRes),
		ID:   msg.ID,
		Ok:   true,
		Payload: protocol.HelloPayload{
			Version:   "0.0.1",
			DeviceID:  client.deviceID,
			SessionID: sessionID,
			Workspace: "default",
			State:     state,
		},
	}

	data, _ := json.Marshal(response)
	client.Conn.Write(data)

	log.Printf("🤝 Handshake complete: device=%s client=%s session=%s", client.deviceID, client.ID, sessionID)

	// Publish connect event
	g.eventBus.Publish(protocol.EventClientConnected, "", map[string]interface{}{
		"client_id":    client.ID,
		"device_id":    client.deviceID,
		"session_id":   sessionID,
		"connected_at": client.connectedAt.Unix(),
	})

	return nil
}

// runHub runs gateway hub
func (g *Gateway) runHub() {
	defer g.wg.Done()

	for {
		select {
		case <-g.ctx.Done():
			return
		case client := <-g.register:
			g.clientsLock.Lock()
			g.clients[client.ID] = client
			g.clientsLock.Unlock()
		case client := <-g.unregister:
			g.clientsLock.Lock()
			if _, ok := g.clients[client.ID]; ok {
				delete(g.clients, client.ID)
			}
			g.clientsLock.Unlock()

			log.Printf("📴 Client disconnected: %s", client.ID)
		case message := <-g.broadcast:
			g.clientsLock.RLock()
			for _, c := range g.clients {
				if err := c.Conn.Write(message); err != nil {
					log.Printf("Broadcast error to %s: %v", c.ID, err)
				}
			}
			g.clientsLock.RUnlock()
		}
	}
}

// GetClient returns a client by ID
func (g *Gateway) GetClient(id string) (*Client, bool) {
	g.clientsLock.RLock()
	defer g.clientsLock.RUnlock()
	client, ok := g.clients[id]
	return client, ok
}

// GetClients returns all clients
func (g *Gateway) GetClients() []*Client {
	g.clientsLock.RLock()
	defer g.clientsLock.RUnlock()

	clients := make([]*Client, 0, len(g.clients))
	for _, client := range g.clients {
		clients = append(clients, client)
	}
	return clients
}

// Broadcast broadcasts a message to all clients
func (g *Gateway) Broadcast(message []byte) {
	select {
	case g.broadcast <- message:
	default:
		log.Printf("Broadcast buffer full, dropping message")
	}
}

// GetEventBus returns the event bus
func (g *Gateway) GetEventBus() *protocol.EventBus {
	return g.eventBus
}

// ID returns the gateway ID
func (g *Gateway) ID() string {
	return g.id
}

// GetState returns current gateway state
func (g *Gateway) GetState() *GatewayState {
	g.clientsLock.RLock()
	defer g.clientsLock.RUnlock()

	stats := &GatewayStats{
		ClientCount: len(g.clients),
		Uptime:      int64(time.Since(time.Now().Add(-time.Hour)).Seconds()),
	}

	return &GatewayState{
		Running: true,
		Version: "0.0.1",
		Stats:  stats,
	}
}
