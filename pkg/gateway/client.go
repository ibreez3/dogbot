package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/openclaw/go-openclaw/internal/protocol"
	"github.com/openclaw/go-openclaw/internal/ws"
)

// Client represents a connected client
type Client struct {
	ID           string             // Unique client ID
	Conn         *ws.Conn          // WebSocket connection
	gateway      *Gateway          // Parent gateway
	deviceID     string            // Device identifier
	clientID     string            // Client identifier
	sessionID    string            // Session identifier
	clientType   string            // Client type: agent, node, web, mobile
	status       string            // Status: connected, disconnected, idle
	connectedAt  time.Time         // Connection time
	lastSeen     time.Time         // Last activity time
	capabilities []string          // Client capabilities
	metadata     map[string]string // Additional metadata
	mu           sync.RWMutex
}

// Manager manages connected clients
type Manager struct {
	gateway *Gateway
	clients map[string]*Client
	lock    sync.RWMutex
}

// NewManager creates a new client manager
func NewManager(gateway *Gateway) *Manager {
	return &Manager{
		gateway: gateway,
		clients: make(map[string]*Client),
	}
}

// Add adds a client
func (m *Manager) Add(client *Client) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.clients[client.ID] = client
}

// Remove removes a client
func (m *Manager) Remove(id string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.clients, id)
}

// Get gets a client by ID
func (m *Manager) Get(id string) (*Client, bool) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	client, ok := m.clients[id]
	return client, ok
}

// GetAll returns all clients
func (m *Manager) GetAll() []*Client {
	m.lock.RLock()
	defer m.lock.RUnlock()

	clients := make([]*Client, 0, len(m.clients))
	for _, client := range m.clients {
		clients = append(clients, client)
	}
	return clients
}

// GetByDeviceID returns clients by device ID
func (m *Manager) GetByDeviceID(deviceID string) []*Client {
	m.lock.RLock()
	defer m.lock.RUnlock()

	var clients []*Client
	for _, client := range m.clients {
		if client.deviceID == deviceID {
			clients = append(clients, client)
		}
	}
	return clients
}

// GetBySessionID returns client by session ID
func (m *Manager) GetBySessionID(sessionID string) (*Client, bool) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	for _, client := range m.clients {
		if client.sessionID == sessionID {
			return client, true
		}
	}
	return nil, false
}

// GetByType returns clients by type
func (m *Manager) GetByType(clientType string) []*Client {
	m.lock.RLock()
	defer m.lock.RUnlock()

	var clients []*Client
	for _, client := range m.clients {
		if client.clientType == clientType {
			clients = append(clients, client)
		}
	}
	return clients
}

// GetActive returns active clients (seen within last 5 minutes)
func (m *Manager) GetActive() []*Client {
	m.lock.RLock()
	defer m.lock.RUnlock()

	threshold := time.Now().Add(-5 * time.Minute)
	var clients []*Client
	for _, client := range m.clients {
		if client.lastSeen.After(threshold) {
			clients = append(clients, client)
		}
	}
	return clients
}

// Count returns the number of clients
func (m *Manager) Count() int {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return len(m.clients)
}

// Broadcast broadcasts a message to all clients
func (m *Manager) Broadcast(msg *protocol.ProtocolMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	m.gateway.Broadcast(data)
	return nil
}

// BroadcastToType broadcasts a message to clients of a specific type
func (m *Manager) BroadcastToType(clientType string, msg *protocol.ProtocolMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	m.lock.RLock()
	defer m.lock.RUnlock()

	for _, client := range m.clients {
		if client.clientType == clientType {
			client.Conn.Write(data)
		}
	}

	return nil
}

// SendTo sends a message to a specific client
func (m *Manager) SendTo(clientID string, msg *protocol.ProtocolMessage) error {
	client, ok := m.Get(clientID)
	if !ok {
		return fmt.Errorf("client not found: %s", clientID)
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return client.Conn.Write(data)
}

// CleanupIdle removes idle clients
func (m *Manager) CleanupIdle(timeout time.Duration) int {
	m.lock.Lock()
	defer m.lock.Unlock()

	threshold := time.Now().Add(-timeout)
	count := 0

	for id, client := range m.clients {
		if client.lastSeen.Before(threshold) {
			client.Close()
			delete(m.clients, id)
			count++
		}
	}

	return count
}

// GetState returns client state
func (c *Client) GetState() *protocol.ClientState {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return &protocol.ClientState{
		ID:           c.ID,
		DeviceID:     c.deviceID,
		Type:         c.clientType,
		Status:       c.status,
		ConnectedAt:  c.connectedAt.Unix(),
		LastSeen:     c.lastSeen.Unix(),
		Capabilities: c.capabilities,
		Metadata:     c.metadata,
	}
}

// Update updates client info
func (c *Client) Update(deviceID, clientID, clientType string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if deviceID != "" {
		c.deviceID = deviceID
	}
	if clientID != "" {
		c.clientID = clientID
	}
	if clientType != "" {
		c.clientType = clientType
	}
}

// SetStatus sets the client status
func (c *Client) SetStatus(status string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = status
}

// SetCapabilities sets the client capabilities
func (c *Client) SetCapabilities(capabilities []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.capabilities = capabilities
}

// SetMetadata sets a metadata value
func (c *Client) SetMetadata(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metadata[key] = value
}

// GetMetadata gets a metadata value
func (c *Client) GetMetadata(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, ok := c.metadata[key]
	return value, ok
}

// Close closes the client connection
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.status = "disconnected"
	return c.Conn.Close()
}

// Send sends a message to the client
func (c *Client) Send(msg *protocol.ProtocolMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return c.Conn.Write(data)
}

// SendRequest sends a request to the client
func (c *Client) SendRequest(id, method string, params interface{}) error {
	return c.Conn.WriteRequest(id, method, params)
}

// SendResponse sends a response to the client
func (c *Client) SendResponse(id string, ok bool, payload interface{}, errorMsg string) error {
	return c.Conn.WriteResponse(id, ok, payload, errorMsg)
}

// SendEvent sends an event to the client
func (c *Client) SendEvent(event string, data interface{}, seq int) error {
	return c.Conn.WriteEvent(event, data, seq)
}

// IsActive checks if the client is active (seen within last 5 minutes)
func (c *Client) IsActive() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return time.Since(c.lastSeen) < 5*time.Minute
}

// IsConnected checks if the client connection is alive
func (c *Client) IsConnected() bool {
	return c.Conn.IsAlive()
}

// SubscribeToEvents subscribes the client to event bus events
func (c *Client) SubscribeToEvents(eventTypes []protocol.EventType, channel string) *protocol.EventSubscriber {
	return c.gateway.GetEventBus().Subscribe(c.ID, channel, eventTypes, nil)
}

// UnsubscribeFromEvents unsubscribes the client from event bus events
func (c *Client) UnsubscribeFromEvents(sub *protocol.EventSubscriber) {
	c.gateway.GetEventBus().Unsubscribe(sub)
}

// CleanupOldSessions removes sessions older than the specified duration
func (c *Client) CleanupOldSessions(maxAge time.Duration) {
	// TODO: Implement session cleanup
}
