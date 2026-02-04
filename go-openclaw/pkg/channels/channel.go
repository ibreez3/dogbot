package channels

import (
	"context"
	"time"
)

// MessageType represents the type of message
type MessageType string

const (
	MessageTypeText  MessageType = "text"
	MessageTypeImage MessageType = "image"
	MessageTypeFile  MessageType = "file"
	MessageTypeAudio MessageType = "audio"
	MessageTypeVideo MessageType = "video"
	MessageTypeSticker MessageType = "sticker"
)

// Message represents a message from/to a channel
type Message struct {
	ID        string                 `json:"id"`
	Channel   string                 `json:"channel"` // telegram, whatsapp, slack, discord, etc.
	From      string                 `json:"from"`    // user ID
	FromName  string                 `json:"from_name,omitempty"`
	To        string                 `json:"to"`      // user ID or group ID
	IsGroup   bool                   `json:"is_group"`
	GroupID   string                 `json:"group_id,omitempty"`
	Content   string                 `json:"content"`
	Type      MessageType            `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	ReplyTo   string                 `json:"reply_to,omitempty"` // message ID being replied to
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// MessageHandler handles incoming messages
type MessageHandler func(ctx context.Context, msg *Message) error

// Channel represents a message channel interface
type Channel interface {
	// Name returns the channel name (e.g., "telegram", "whatsapp")
	Name() string

	// Start starts the channel and begins listening for messages
	Start(ctx context.Context) error

	// Stop stops the channel
	Stop(ctx context.Context) error

	// Send sends a message to the specified target
	Send(ctx context.Context, target string, content string, options map[string]interface{}) error

	// SetMessageHandler sets the handler for incoming messages
	SetMessageHandler(handler MessageHandler)

	// IsRunning returns true if the channel is running
	IsRunning() bool

	// Status returns the current status of the channel
	Status() string
}

// ChannelConfig represents configuration for a channel
type ChannelConfig struct {
	Enabled bool                   `json:"enabled"`
	Config  map[string]interface{} `json:"config"`
}

// ChannelManager manages multiple channels
type ChannelManager struct {
	channels map[string]Channel
}

// NewChannelManager creates a new channel manager
func NewChannelManager() *ChannelManager {
	return &ChannelManager{
		channels: make(map[string]Channel),
	}
}

// Register registers a channel
func (cm *ChannelManager) Register(channel Channel) {
	cm.channels[channel.Name()] = channel
}

// Get returns a channel by name
func (cm *ChannelManager) Get(name string) (Channel, bool) {
	ch, ok := cm.channels[name]
	return ch, ok
}

// GetAll returns all registered channels
func (cm *ChannelManager) GetAll() []Channel {
	channels := make([]Channel, 0, len(cm.channels))
	for _, ch := range cm.channels {
		channels = append(channels, ch)
	}
	return channels
}

// StartAll starts all channels
func (cm *ChannelManager) StartAll(ctx context.Context) error {
	for _, ch := range cm.channels {
		if err := ch.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}

// StopAll stops all channels
func (cm *ChannelManager) StopAll(ctx context.Context) error {
	for _, ch := range cm.channels {
		if err := ch.Stop(ctx); err != nil {
			return err
		}
	}
	return nil
}
