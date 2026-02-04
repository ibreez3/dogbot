package gateway

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/openclaw/go-openclaw/internal/protocol"
)

// HeartbeatConfig contains heartbeat configuration
type HeartbeatConfig struct {
	PingInterval   time.Duration // Interval between pings
	PongTimeout    time.Duration // Timeout for pong response
	IdleTimeout    time.Duration // Timeout for idle connections
	CheckInterval  time.Duration // Interval for idle checks
}

// DefaultHeartbeatConfig returns the default heartbeat configuration
func DefaultHeartbeatConfig() *HeartbeatConfig {
	return &HeartbeatConfig{
		PingInterval:  54 * time.Second,  // Send ping every 54s
		PongTimeout:   60 * time.Second,  // Expect pong within 60s
		IdleTimeout:   5 * time.Minute,   // Disconnect idle after 5 min
		CheckInterval: 1 * time.Minute,   // Check idle every 1 min
	}
}

// Heartbeat manages heartbeat and idle detection
type Heartbeat struct {
	gateway *Gateway
	config  *HeartbeatConfig
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	seq     int
	lock    sync.Mutex
}

// NewHeartbeat creates a new heartbeat manager
func NewHeartbeat(gateway *Gateway, config *HeartbeatConfig) *Heartbeat {
	if config == nil {
		config = DefaultHeartbeatConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Heartbeat{
		gateway: gateway,
		config:  config,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start starts the heartbeat manager
func (h *Heartbeat) Start() {
	// Start ping loop
	h.wg.Add(1)
	go h.pingLoop()

	// Start idle check loop
	h.wg.Add(1)
	go h.idleCheckLoop()
}

// Stop stops the heartbeat manager
func (h *Heartbeat) Stop() {
	h.cancel()
	h.wg.Wait()
}

// pingLoop sends ping messages periodically
func (h *Heartbeat) pingLoop() {
	defer h.wg.Done()

	ticker := time.NewTicker(h.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.sendPingToAll()

		case <-h.ctx.Done():
			return
		}
	}
}

// sendPingToAll sends ping to all connected clients
func (h *Heartbeat) sendPingToAll() {
	clients := h.gateway.GetClients()

	for _, client := range clients {
		if !client.IsConnected() {
			continue
		}

		if err := h.sendPing(client); err != nil {
			log.Printf("Failed to send ping to %s: %v", client.ID, err)
		}
	}
}

// sendPing sends a ping to a specific client
func (h *Heartbeat) sendPing(client *Client) error {
	h.lock.Lock()
	h.seq++
	seq := h.seq
	h.lock.Unlock()

	// Use WebSocket ping (more efficient than protocol-level ping)
	client.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := client.Conn.WriteRaw([]byte{}); err != nil {
		return err
	}

	return nil
}

// idleCheckLoop checks for idle clients periodically
func (h *Heartbeat) idleCheckLoop() {
	defer h.wg.Done()

	ticker := time.NewTicker(h.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.checkIdleClients()

		case <-h.ctx.Done():
			return
		}
	}
}

// checkIdleClients checks and disconnects idle clients
func (h *Heartbeat) checkIdleClients() {
	clients := h.gateway.GetClients()
	now := time.Now()

	for _, client := range clients {
		lastSeen := client.lastSeen()

		// Check if client is idle
		if now.Sub(lastSeen) > h.config.IdleTimeout {
			log.Printf("Client %s idle, disconnecting (last seen: %v)", client.ID, lastSeen)
			client.Close()
		}
	}
}

// IsClientAlive checks if a client is alive (recent activity)
func (h *Heartbeat) IsClientAlive(client *Client) bool {
	lastSeen := client.lastSeen()
	return time.Since(lastSeen) < h.config.PongTimeout
}

// GetAliveClients returns all alive clients
func (h *Heartbeat) GetAliveClients() []*Client {
	clients := h.gateway.GetClients()
	alive := make([]*Client, 0, len(clients))

	for _, client := range clients {
		if h.IsClientAlive(client) {
			alive = append(alive, client)
		}
	}

	return alive
}

// GetDeadClients returns all dead clients
func (h *Heartbeat) GetDeadClients() []*Client {
	clients := h.gateway.GetClients()
	dead := make([]*Client, 0)

	for _, client := range clients {
		if !h.IsClientAlive(client) {
			dead = append(dead, client)
		}
	}

	return dead
}

// PingClient sends a ping to a specific client and waits for pong
func (h *Heartbeat) PingClient(client *Client) error {
	h.lock.Lock()
	h.seq++
	seq := h.seq
	h.lock.Unlock()

	// Create ping request
	pingMsg := &protocol.ProtocolMessage{
		Type:   protocol.TypeReq,
		ID:     generateID(),
		Method: "ping",
		Params: nil, // No params needed
	}

	return client.Send(pingMsg)
}

// HandlePong handles a pong message from a client
func (h *Heartbeat) HandlePong(client *Client) {
	client.lastSeen = time.Now()
}

// GetStats returns heartbeat statistics
func (h *Heartbeat) GetStats() *HeartbeatStats {
	clients := h.gateway.GetClients()
	now := time.Now()

	stats := &HeartbeatStats{
		TotalClients: len(clients),
		AliveClients: 0,
		IdleClients:  0,
	}

	for _, client := range clients {
		lastSeen := client.lastSeen()

		if time.Since(lastSeen) < h.config.PongTimeout {
			stats.AliveClients++
		} else {
			stats.IdleClients++
		}
	}

	return stats
}

// HeartbeatStats contains heartbeat statistics
type HeartbeatStats struct {
	TotalClients int       `json:"total_clients"`
	AliveClients int       `json:"alive_clients"`
	IdleClients  int       `json:"idle_clients"`
	Timestamp    time.Time `json:"timestamp"`
}

// PublishHeartbeatEvent publishes a heartbeat event
func (h *Heartbeat) PublishHeartbeatEvent() {
	stats := h.GetStats()

	data, _ := json.Marshal(stats)
	h.gateway.GetEventBus().PublishData(
		protocol.EventCustom,
		"",
		data,
	)
}

// BroadcastHeartbeatState broadcasts the current heartbeat state
func (h *Heartbeat) BroadcastHeartbeatState() error {
	stats := h.GetStats()

	msg := &protocol.ProtocolMessage{
		Type:  protocol.TypeEvent,
		Event: "heartbeat.state",
		Data:  stats,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	h.gateway.Broadcast(data)
	return nil
}

// helper function to generate ID
func generateID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(6)
}

// helper function to generate random string
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().Nanosecond()%len(charset)]
	}
	return string(b)
}
