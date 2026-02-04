package events

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/openclaw/go-openclaw/internal/protocol"
	"go.uber.org/zap"
)

// EventType represents the type of event
type EventType string

const (
	EventAgent     EventType = "agent"
	EventChat      EventType = "chat"
	EventPresence  EventType = "presence"
	EventHealth    EventType = "health"
	EventNode      EventType = "node"
	EventWorkspace EventType = "workspace"
)

// Event represents an internal event
type Event struct {
	Type      EventType       `json:"type"`
	Name      string          `json:"name"`
	Data      json.RawMessage `json:"data"`
	Timestamp time.Time       `json:"timestamp"`
	Source    string          `json:"source"`
	Metadata  map[string]any  `json:"metadata,omitempty"`
}

// Handler is a function that handles events
type Handler func(ctx context.Context, event *Event) error

// EventBus manages event subscription and broadcasting
type EventBus struct {
	handlers map[EventType][]Handler
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	logger   *zap.Logger
}

// New creates a new event bus
func New(logger *zap.Logger) *EventBus {
	ctx, cancel := context.WithCancel(context.Background())
	return &EventBus{
		handlers: make(map[EventType][]Handler),
		ctx:      ctx,
		cancel:   cancel,
		logger:   logger,
	}
}

// Subscribe subscribes a handler to an event type
func (eb *EventBus) Subscribe(eventType EventType, handler Handler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.handlers[eventType] = append(eb.handlers[eventType], handler)
	eb.logger.Debug("Handler subscribed",
		zap.String("event_type", string(eventType)),
		zap.Int("total_handlers", len(eb.handlers[eventType])))
}

// Publish publishes an event to all subscribers
func (eb *EventBus) Publish(ctx context.Context, event *Event) error {
	eb.mu.RLock()
	handlers := make([]Handler, 0, len(eb.handlers[event.Type]))
	for _, h := range eb.handlers[event.Type] {
		handlers = append(handlers, h)
	}
	eb.mu.RUnlock()

	eb.logger.Debug("Event published",
		zap.String("event_type", string(event.Type)),
		zap.String("event_name", event.Name),
		zap.Int("handlers", len(handlers)))

	// Call all handlers
	var errs []error
	for _, handler := range handlers {
		if err := handler(ctx, event); err != nil {
			eb.logger.Error("Handler error",
				zap.String("event_type", string(event.Type)),
				zap.String("event_name", event.Name),
				zap.Error(err))
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("some handlers failed: %d errors", len(errs))
	}

	return nil
}

// PublishAsync publishes an event asynchronously
func (eb *EventBus) PublishAsync(event *Event) {
	go func() {
		if err := eb.Publish(eb.ctx, event); err != nil {
			eb.logger.Error("Async event publish failed", zap.Error(err))
		}
	}()
}

// Stop stops the event bus
func (eb *EventBus) Stop() {
	eb.cancel()
}

// NewEvent creates a new event
func NewEvent(eventType, name string, data any, source string) (*Event, error) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event data: %w", err)
	}

	return &Event{
		Type:      EventType(eventType),
		Name:      name,
		Data:      dataBytes,
		Timestamp: time.Now(),
		Source:    source,
		Metadata:  make(map[string]any),
	}, nil
}

// UnmarshalData unmarshals the event data to the given type
func (e *Event) UnmarshalData(v any) error {
	return json.Unmarshal(e.Data, v)
}

// ToProtocolMessage converts the event to a protocol message
func (e *Event) ToProtocolMessage(seq int) *protocol.ProtocolMessage {
	return &protocol.ProtocolMessage{
		Type:  protocol.TypeEvent,
		Seq:   seq,
		Event: string(e.Type) + "/" + e.Name,
		Data:  e.Data,
	}
}

// AgentEventData represents agent event data
type AgentEventData struct {
	Action   string                 `json:"action"`   // connect, disconnect, heartbeat, status
	AgentID  string                 `json:"agent_id"`
	NodeID   string                 `json:"node_id,omitempty"`
	Details  map[string]any         `json:"details,omitempty"`
	Metadata map[string]string      `json:"metadata,omitempty"`
}

// ChatEventData represents chat event data
type ChatEventData struct {
	ChannelID   string            `json:"channel_id"`
	MessageID   string            `json:"message_id,omitempty"`
	Action      string            `json:"action"` // send, receive, edit, delete
	Content     string            `json:"content,omitempty"`
	From        string            `json:"from,omitempty"`
	To          string            `json:"to,omitempty"`
	Timestamp   time.Time         `json:"timestamp,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// PresenceEventData represents presence event data
type PresenceEventData struct {
	UserID   string            `json:"user_id"`
	Status   string            `json:"status"` // online, offline, away, busy
	NodeID   string            `json:"node_id,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// HealthEventData represents health event data
type HealthEventData struct {
	Component string            `json:"component"` // gateway, node, storage, etc.
	Status    string            `json:"status"`    // ok, degraded, error
	Metrics   map[string]any    `json:"metrics,omitempty"`
	Message   string            `json:"message,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// NodeEventData represents node event data
type NodeEventData struct {
	NodeID    string            `json:"node_id"`
	Action    string            `json:"action"` // connect, disconnect, heartbeat, status
	Name      string            `json:"name,omitempty"`
	Type      string            `json:"type,omitempty"` // mobile, desktop, server
	Location  string            `json:"location,omitempty"`
	IPAddress string            `json:"ip_address,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}
