package models

import (
	"time"
)

// Client represents a connected client
type Client struct {
	ID           string                 `json:"id"`
	DeviceID     string                 `json:"device_id"`
	SessionID    string                 `json:"session_id"`
	Type         string                 `json:"type"` // agent, node, web, mobile
	Status       string                 `json:"status"` // connected, disconnected
	ConnectedAt  time.Time              `json:"connected_at"`
	DisconnectedAt *time.Time           `json:"disconnected_at,omitempty"`
	LastSeen     time.Time              `json:"last_seen"`
	Capabilities []string               `json:"capabilities"`
	Metadata     map[string]string      `json:"metadata"`
}

// Node represents a paired device node
type Node struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Type         string            `json:"type"` // mobile, desktop, server
	Status       string            `json:"status"` // online, offline, error
	IPAddress    string            `json:"ip_address"`
	Location     string            `json:"location,omitempty"`
	PairedAt     time.Time         `json:"paired_at"`
	LastSeen     time.Time         `json:"last_seen"`
	Capabilities []string          `json:"capabilities"`
	Metadata     map[string]string `json:"metadata"`
}

// Agent represents an agent instance
type Agent struct {
	ID           string            `json:"id"`
	SessionID    string            `json:"session_id"`
	NodeID       string            `json:"node_id,omitempty"`
	ChannelID    string            `json:"channel_id,omitempty"`
	Status       string            `json:"status"` // active, idle, stopped
	Capabilities []string          `json:"capabilities"`
	StartedAt    time.Time         `json:"started_at"`
	LastActive   time.Time         `json:"last_active"`
	Metadata     map[string]string `json:"metadata"`
}

// Workspace represents a workspace
type Workspace struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Settings    map[string]any    `json:"settings"`
	Metadata    map[string]string `json:"metadata"`
}

// Session represents a user session
type Session struct {
	ID         string            `json:"id"`
	ClientID   string            `json:"client_id"`
	DeviceID   string            `json:"device_id"`
	UserID     string            `json:"user_id,omitempty"`
	Workspace  string            `json:"workspace"`
	Channel    string            `json:"channel,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	ExpiresAt  time.Time         `json:"expires_at,omitempty"`
	Metadata   map[string]string `json:"metadata"`
}

// Message represents a message
type Message struct {
	ID         string            `json:"id"`
	ChannelID  string            `json:"channel_id"`
	From       string            `json:"from"`
	To         string            `json:"to,omitempty"`
	Content    string            `json:"content"`
	Type       string            `json:"type"` // text, image, file, etc.
	Timestamp  time.Time         `json:"timestamp"`
	Metadata   map[string]string `json:"metadata"`
}

// EventLog represents a logged event
type EventLog struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Name      string                 `json:"name"`
	Data      map[string]interface{} `json:"data"`
	Source    string                 `json:"source"`
	Timestamp time.Time              `json:"timestamp"`
}

// HealthStatus represents health status
type HealthStatus struct {
	Component string                 `json:"component"`
	Status    string                 `json:"status"` // ok, degraded, error
	Message   string                 `json:"message,omitempty"`
	Metrics   map[string]interface{} `json:"metrics,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}
