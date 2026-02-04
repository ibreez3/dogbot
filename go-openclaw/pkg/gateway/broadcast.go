package gateway

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/openclaw/go-openclaw/internal/protocol"
)

// Broadcaster manages event broadcasting to clients
type Broadcaster struct {
	gateway  *Gateway
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	snapshot *StateSnapshot
	lock     sync.RWMutex
}

// StateSnapshot represents a cached state snapshot
type StateSnapshot struct {
	state     *protocol.StateSnapshot
	timestamp time.Time
}

// NewBroadcaster creates a new broadcaster
func NewBroadcaster(gateway *Gateway) *Broadcaster {
	ctx, cancel := context.WithCancel(context.Background())

	return &Broadcaster{
		gateway:  gateway,
		ctx:      ctx,
		cancel:   cancel,
		snapshot: &StateSnapshot{},
	}
}

// Start starts the broadcaster
func (b *Broadcaster) Start() {
	// Start snapshot broadcaster
	b.wg.Add(1)
	go b.snapshotBroadcastLoop()

	// Start event loop
	b.wg.Add(1)
	go b.eventLoop()
}

// Stop stops the broadcaster
func (b *Broadcaster) Stop() {
	b.cancel()
	b.wg.Wait()
}

// eventLoop listens to event bus and broadcasts to clients
func (b *Broadcaster) eventLoop() {
	defer b.wg.Done()

	eventBus := b.gateway.GetEventBus()

	// Create temporary subscriber for internal events
	sub := eventBus.Subscribe("broadcaster", "", []protocol.EventType{
		protocol.EventClientConnected,
		protocol.EventClientDisconnected,
		protocol.EventClientUpdate,
		protocol.EventSessionCreated,
		protocol.EventSessionClosed,
		protocol.EventStateUpdate,
	}, nil)

	defer eventBus.Unsubscribe(sub)

	eventChan := eventBus.GetSubscriberChannel(sub)

	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				return
			}

			b.broadcastEvent(event)

		case <-b.ctx.Done():
			return
		}
	}
}

// broadcastEvent broadcasts an event to matching clients
func (b *Broadcaster) broadcastEvent(event *protocol.Event) {
	// Create protocol message
	msg := &protocol.ProtocolMessage{
		Type:  protocol.TypeEvent,
		Event: string(event.Type),
		Data:  event.Data,
		Seq:   event.Seq,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal event: %v", err)
		return
	}

	// Get clients
	clients := b.gateway.GetClients()

	// Filter and send
	for _, client := range clients {
		// Check channel filter
		if event.Channel != "" && client.deviceID != event.Channel {
			continue
		}

		// Check if client is connected
		if !client.IsConnected() {
			continue
		}

		// Send event
		if err := client.Conn.Write(data); err != nil {
			log.Printf("Failed to send event to %s: %v", client.ID, err)
		}
	}
}

// snapshotBroadcastLoop periodically broadcasts state snapshots
func (b *Broadcaster) snapshotBroadcastLoop() {
	defer b.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.broadcastStateSnapshot()

		case <-b.ctx.Done():
			return
		}
	}
}

// broadcastStateSnapshot broadcasts the current state snapshot
func (b *Broadcaster) broadcastStateSnapshot() {
	state := b.gateway.GetState()
	if state == nil {
		return
	}

	msg := &protocol.ProtocolMessage{
		Type:  protocol.TypeEvent,
		Event: string(protocol.EventStateUpdate),
		Data:  state,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal state snapshot: %v", err)
		return
	}

	b.gateway.Broadcast(data)
}

// Broadcast broadcasts a message to all clients
func (b *Broadcaster) Broadcast(msg *protocol.ProtocolMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	b.gateway.Broadcast(data)
	return nil
}

// BroadcastToType broadcasts to clients of a specific type
func (b *Broadcaster) BroadcastToType(clientType string, msg *protocol.ProtocolMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	clients := b.gateway.GetClients()
	for _, client := range clients {
		if client.clientType == clientType {
			client.Conn.Write(data)
		}
	}

	return nil
}

// BroadcastToChannel broadcasts to clients on a specific channel
func (b *Broadcaster) BroadcastToChannel(channel string, msg *protocol.ProtocolMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	clients := b.gateway.GetClients()
	for _, client := range clients {
		if client.deviceID == channel || client.sessionID == channel {
			client.Conn.Write(data)
		}
	}

	return nil
}

// PublishEvent publishes an event to the event bus
func (b *Broadcaster) PublishEvent(eventType protocol.EventType, channel string, data interface{}) {
	b.gateway.GetEventBus().Publish(eventType, channel, data)
}

// GetSnapshot returns the cached state snapshot
func (b *Broadcaster) GetSnapshot() *protocol.StateSnapshot {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.snapshot.state
}

// UpdateSnapshot updates the cached state snapshot
func (b *Broadcaster) UpdateSnapshot() {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.snapshot.state = b.gateway.GetState()
	b.snapshot.timestamp = time.Now()
}

// IsSnapshotStale checks if the cached snapshot is stale
func (b *Broadcaster) IsSnapshotStale(maxAge time.Duration) bool {
	b.lock.RLock()
	defer b.lock.RUnlock()

	if b.snapshot.state == nil {
		return true
	}

	return time.Since(b.snapshot.timestamp) > maxAge
}
