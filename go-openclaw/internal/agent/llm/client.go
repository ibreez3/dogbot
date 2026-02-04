package llm

import (
	"context"
)

// Provider represents an LLM provider
type Provider string

const (
	ProviderAnthropic Provider = "anthropic"
	ProviderOpenAI     Provider = "openai"
	ProviderOpenRouter  Provider = "openrouter"
	ProviderUnknown    Provider = "unknown"
)

// String returns string representation
func (p Provider) String() string {
	return string(p)
}

// Client represents an LLM client interface
type Client interface {
	// Basic operations
	Provider() Provider
	Model() string
	
	// Messaging
	SendMessage(ctx context.Context, req *Request) (*Response, error)
	SendMessages(ctx context.Context, req *MultiMessageRequest) (*Response, error)
	
	// Streaming
	StreamMessage(ctx context.Context, req *Request, handler StreamHandler) error
	
	// Tools
	CallTool(ctx context.Context, tool string, params map[string]interface{}) (*ToolResponse, error)
	
	// Close
	Close(ctx context.Context) error
}

// Request represents a generic LLM request
type Request struct {
	SystemPrompt string            `json:"system_prompt,omitempty"`
	Messages     []Message            `json:"messages"`
	MaxTokens    int                  `json:"max_tokens,omitempty"`
	Temperature  float64             `json:"temperature,omitempty"`
	TopP         int                  `json:"top_p,omitempty"`
	Stream       bool                 `json:"stream,omitempty"`
	Tools        []Tool                `json:"tools,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// Message represents a message in the conversation
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// MultiMessageRequest represents a request with multiple messages
type MultiMessageRequest struct {
	Messages []Message `json:"messages"`
	// Inherits other fields from Request
}

// Response represents an LLM response
type Response struct {
	Text     string                 `json:"text"`
	Usage    *Usage                  `json:"usage,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Usage represents token usage information
type Usage struct {
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TotalTokens  int     `json:"total_tokens"`
}

// Tool represents a tool definition
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// ToolResponse represents a tool call response
type ToolResponse struct {
	Name   string                 `json:"name"`
	Params map[string]interface{} `json:"params"`
}

// StreamHandler handles streaming responses
type StreamHandler func(chunk string, done bool) error

// ErrorResponse represents an error response
type ErrorResponse struct {
	Message string `json:"message"`
	Type    string `json:"type,omitempty"`
}
