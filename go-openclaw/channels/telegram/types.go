package channels

import (
	"time"
)

// UserInfo represents Telegram user information
type UserInfo struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
	IsBot     bool   `json:"is_bot"`
}

// String returns the full name or username
func (u *UserInfo) String() string {
	if u == nil {
		return "Unknown"
	}
	if u.Username != "" {
		return "@" + u.Username
	}
	if u.LastName != "" {
		return u.FirstName + " " + u.LastName
	}
	return u.FirstName
}

// ChatInfo represents Telegram chat information
type ChatInfo struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"` // private, group, supergroup, channel
	Title     string `json:"title,omitempty"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

// String returns a string representation of the chat
func (c *ChatInfo) String() string {
	if c == nil {
		return "Unknown"
	}
	switch c.Type {
	case "private":
		if c.Username != "" {
			return "@" + c.Username
		}
		if c.LastName != "" {
			return c.FirstName + " " + c.LastName
		}
		return c.FirstName
	case "group", "supergroup", "channel":
		if c.Title != "" {
			return c.Title
		}
		if c.Username != "" {
			return "@" + c.Username
		}
		return "Group " + string(c.ID)
	default:
		return "Chat " + string(c.ID)
	}
}

// IsGroup returns true if the chat is a group or supergroup
func (c *ChatInfo) IsGroup() bool {
	if c == nil {
		return false
	}
	return c.Type == "group" || c.Type == "supergroup" || c.Type == "channel"
}

// IncomingMessage represents an incoming Telegram message
type IncomingMessage struct {
	MessageID   int64       `json:"message_id"`
	From        *UserInfo   `json:"from"`
	Chat        *ChatInfo   `json:"chat"`
	Text        string      `json:"text,omitempty"`
	Caption     string      `json:"caption,omitempty"`
	ReplyTo     *IncomingMessage `json:"reply_to_message,omitempty"`
	Date        time.Time   `json:"date"`
	EditDate    *time.Time  `json:"edit_date,omitempty"`
	MediaGroupID string     `json:"media_group_id,omitempty"`
}

// GetContent returns the text or caption of the message
func (m *IncomingMessage) GetContent() string {
	if m == nil {
		return ""
	}
	if m.Text != "" {
		return m.Text
	}
	if m.Caption != "" {
		return m.Caption
	}
	return ""
}

// OutgoingMessage represents an outgoing Telegram message
type OutgoingMessage struct {
	ChatID      int64       `json:"chat_id"`
	Text        string      `json:"text"`
	ParseMode   string      `json:"parse_mode,omitempty"` // Markdown, MarkdownV2, HTML
	ReplyTo     int64       `json:"reply_to_message_id,omitempty"`
	DisableWebPagePreview bool `json:"disable_web_page_preview,omitempty"`
	DisableNotification   bool `json:"disable_notification,omitempty"`
}

// SendOptions represents options for sending a message
type SendOptions struct {
	ParseMode              string `json:"parse_mode,omitempty"`
	ReplyTo                int64  `json:"reply_to_message_id,omitempty"`
	DisableWebPagePreview  bool   `json:"disable_web_page_preview,omitempty"`
	DisableNotification    bool   `json:"disable_notification,omitempty"`
}
