package protocol

import (
	"encoding/json"
	"sync"
)

// EventType represents the type of event
type EventType string

const (
	// Client events
	EventClientConnected    EventType = "client.connected"
	EventClientDisconnected EventType = "client.disconnected"
	EventClientUpdate       EventType = "client.update"

	// Session events
	EventSessionCreated EventType = "session.created"
	EventSessionClosed EventType = "session.closed"
	EventSessionUpdate EventType = "session.update"

	// Agent events
	EventAgentStarted EventType = "agent.started"
	EventAgentStopped EventType = "agent.stopped"
	EventAgentMessage EventType = "agent.message"
	EventAgentError   EventType = "agent.error"

	// State events
	EventStateUpdate EventType = "state.update"

	// Custom events
	EventCustom EventType = "custom"
)

// Event represents a broadcast event
type Event struct {
	Type    EventType       `json:"type"`
	Channel string          `json:"channel,omitempty"` // Channel filter (optional)
	Data    json.RawMessage `json:"data"`
	Seq     int             `json:"seq"`
	Time    int64           `json:"timestamp"`
}

// EventBus manages event broadcasting
type EventBus struct {
	subscribers map[*EventSubscriber]bool
	mu          sync.RWMutex
	seq         int
}

// EventSubscriber represents an event subscriber
type EventSubscriber struct {
	ID          string
	Channel     string      // Channel filter (empty = all channels)
	EventTypes  []EventType // Event type filter (empty = all types)
	Callback    func(*Event) bool // Returns true to continue subscription
	send        chan *Event
}

// NewEventBus creates a new event bus
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[*EventSubscriber]bool),
	}
}

// Subscribe subscribes to events
func (eb *EventBus) Subscribe(id, channel string, eventTypes []EventType, callback func(*Event) bool) *EventSubscriber {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	sub := &EventSubscriber{
		ID:         id,
		Channel:    channel,
		EventTypes: eventTypes,
		Callback:   callback,
		send:       make(chan *Event, 256),
	}

	eb.subscribers[sub] = true
	return sub
}

// Unsubscribe removes a subscriber
func (eb *EventBus) Unsubscribe(sub *EventSubscriber) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if _, ok := eb.subscribers[sub]; ok {
		delete(eb.subscribers, sub)
		close(sub.send)
	}
}

// Publish publishes an event
func (eb *EventBus) Publish(eventType EventType, channel string, data interface{}) {
	var dataRaw json.RawMessage
	if data != nil {
		d, err := json.Marshal(data)
		if err != nil {
			return
		}
		dataRaw = d
	}

	eb.mu.Lock()
	eb.seq++
	seq := eb.seq
	eb.mu.Unlock()

	event := &Event{
		Type:    eventType,
		Channel: channel,
		Data:    dataRaw,
		Seq:     seq,
		Time:    getCurrentTimestamp(),
	}

	// Send to matching subscribers
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	for sub := range eb.subscribers {
		if !eb.matchFilter(sub, eventType, channel) {
			continue
		}

		select {
		case sub.send <- event:
			// Event sent
		default:
			// Channel full, try callback directly
			if sub.Callback != nil && !sub.Callback(event) {
				// Subscriber wants to unsubscribe
				delete(eb.subscribers, sub)
			}
		}
	}
}

// PublishData publishes an event with raw JSON data
func (eb *EventBus) PublishData(eventType EventType, channel string, data json.RawMessage) {
	eb.mu.Lock()
	eb.seq++
	seq := eb.seq
	eb.mu.Unlock()

	event := &Event{
		Type:    eventType,
		Channel: channel,
		Data:    data,
		Seq:     seq,
		Time:    getCurrentTimestamp(),
	}

	// Send to matching subscribers
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	for sub := range eb.subscribers {
		if !eb.matchFilter(sub, eventType, channel) {
			continue
		}

		select {
		case sub.send <- event:
		default:
			if sub.Callback != nil && !sub.Callback(event) {
				delete(eb.subscribers, sub)
			}
		}
	}
}

// matchFilter checks if a subscriber matches the event
func (eb *EventBus) matchFilter(sub *EventSubscriber, eventType EventType, channel string) bool {
	// Check channel filter
	if sub.Channel != "" && sub.Channel != channel {
		return false
	}

	// Check event type filter
	if len(sub.EventTypes) > 0 {
		matched := false
		for _, t := range sub.EventTypes {
			if t == eventType {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

// GetSubscriberChannel returns the subscriber's event channel
func (eb *EventBus) GetSubscriberChannel(sub *EventSubscriber) <-chan *Event {
	return sub.send
}

// GetSubscriberCount returns the number of active subscribers
func (eb *EventBus) GetSubscriberCount() int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	return len(eb.subscribers)
}

// Helper function to get current timestamp
func getCurrentTimestamp() int64 {
	// Return milliseconds since epoch
	return 0 // Placeholder, will be implemented in gateway
}
