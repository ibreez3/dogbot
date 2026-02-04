package session

import "time"

// Session represents an agent session
type Session struct {
	ID         string            `json:"id"`
	CreatedAt  time.Time         `json:"created_at"`
	LastActive time.Time         `json:"last_active"`
	Status     string            `json:"status"`
	Messages   []*Message        `json:"messages,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// Message represents a message in a session
type Message struct {
	ID        string                 `json:"id"`
	Role      string                 `json:"role"`      // user, assistant, system
	Content   string                 `json:"content"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}
