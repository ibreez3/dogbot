package agent

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/openclaw/go-openclaw/internal/agent/llm"
	"github.com/openclaw/go-openclaw/internal/agent/tools"
	"github.com/openclaw/go-openclaw/internal/agent/session"
	"github.com/openclaw/go-openclaw/pkg/channels"
)

// Runtime represents Agent runtime
type Runtime struct {
	llm          llm.Client
	toolRegistry *tools.Registry
	sessionMgr   session.Manager
	running      bool
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// Config represents Agent runtime configuration
type Config struct {
	LLMProvider   string            `mapstructure:"llm_provider"`
	LLMModel      string            `mapstructure:"llm_model"`
	MaxTokens     int               `mapstructure:"max_tokens"`
	Temperature    float64           `mapstructure:"temperature"`
	TopP          int               `mapstructure:"top_p"`
	Timeout       time.Duration      `mapstructure:"timeout"`
	SystemPrompt   string            `mapstructure:"system_prompt"`
	ToolsEnabled  bool              `mapstructure:"tools_enabled"`
}

// DefaultConfig returns default Agent configuration
func DefaultConfig() *Config {
	return &Config{
		LLMProvider:   "anthropic",
		LLMModel:      "claude-3-5-sonnet-20241022",
		MaxTokens:     200000,
		Temperature:    0.7,
		TopP:          5,
		Timeout:       30 * time.Second,
		SystemPrompt:   "You are OpenClaw, an AI agent platform built to help developers build intelligent applications.\n\nYour role is to assist users with their development tasks, answer questions, and provide helpful suggestions.\n\nBe concise, practical, and focus on technical accuracy.",
		ToolsEnabled:  false, // Will be enabled later
	}
}

// NewRuntime creates a new Agent runtime
func NewRuntime(config *Config) (*Runtime, error) {
	// Create LLM client
	var llmClient llm.Client
	var err error

	switch config.LLMProvider {
	case "anthropic":
		llmClient, err = llm.NewAnthropicClient(config.LLMModel, config.MaxTokens)
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", config.LLMProvider)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client: %w", err)
	}

	// Create tool registry
	toolRegistry := tools.NewRegistry()

	// Create session manager
	sessionMgr, err := session.NewManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create session manager: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Runtime{
		llm:          llmClient,
		toolRegistry:  toolRegistry,
		sessionMgr:   sessionMgr,
		running:      false,
		ctx:          ctx,
		cancel:       cancel,
		wg:           sync.WaitGroup{},
	}
}

// Start starts the Agent runtime
func (r *Runtime) Start() error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return fmt.Errorf("runtime is already running")
	}

	r.running = true
	r.mu.Unlock()

	log.Printf("🤖 Starting Agent runtime (provider=%s, model=%s)...",
		r.llm.Provider(), r.llm.Model())

	// Start tool registry
	if err := r.toolRegistry.Start(r.ctx); err != nil {
		return fmt.Errorf("failed to start tool registry: %w", err)
	}

	// Start session manager
	if err := r.sessionMgr.Start(r.ctx); err != nil {
		return fmt.Errorf("failed to start session manager: %w", err)
	}

	r.wg.Add(1)
	go r.eventLoop()

	log.Printf("✅ Agent runtime started")

	return nil
}

// Stop stops the Agent runtime
func (r *Runtime) Stop(ctx context.Context) error {
	log.Println("🛑 Stopping Agent runtime...")

	r.cancel()

	// Stop tool registry
	r.toolRegistry.Stop()

	// Stop session manager
	if err := r.sessionMgr.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop session manager: %w", err)
	}

	r.wg.Wait()

	log.Printf("✅ Agent runtime stopped")

	return nil
}

// ProcessMessage processes a message from a channel and returns LLM response
func (r *Runtime) ProcessMessage(ctx context.Context, channel channels.Channel, msg *channels.Message) (string, error) {
	// Get or create session for this channel
	session, err := r.sessionMgr.GetOrCreate(channel.ID)
	if err != nil {
		return "", fmt.Errorf("failed to get session: %w", err)
	}

	// Build message history for context
	history := r.buildMessageHistory(session, msg)

	// Build user prompt
	systemPrompt := r.buildSystemPrompt()

	// Prepare request
	llmReq := r.buildLLMRequest(msg, history, systemPrompt)

	// Call LLM
	llmResp, err := r.llm.SendMessage(ctx, llmReq)
	if err != nil {
		return "", fmt.Errorf("LLM call failed: %w", err)
	}

	// Extract and stream response
	response, err := r.extractResponse(llmResp)
	if err != nil {
		return "", fmt.Errorf("response extraction failed: %w", err)
	}

	// Update session history
	session.AddMessage(&session.Message{
		Role:     "assistant",
		Content:  response.Text,
		Timestamp: time.Now(),
	})

	return response.Text, nil
}

// buildMessageHistory builds message history for LLM context
func (r *Runtime) buildMessageHistory(session *session.Session, msg *channels.Message) []llm.Message {
	// Get recent messages (last 10)
	recent := session.GetMessages(10)
	if len(recent) == 0 {
		return make([]llm.Message, 0)
	}

	// Convert to LLM message format
	history := make([]llm.Message, 0, len(recent))
	for i, msg := range recent {
		// LLM format: Role (user/assistant), Content
		role := "user"
		if msg.FromName == session.ID {
			role = "assistant"
		}
		history[i] = llm.Message{
			Role:    role,
			Content: msg.Content,
		}
	}

	return history
}

// buildSystemPrompt builds system prompt with context
func (r *Runtime) buildSystemPrompt() string {
	// Build context from sessions
	// TODO: Include session metadata, workspace info, etc.

	// For now, use a simple system prompt
	prompt := "You are OpenClaw, an AI agent platform built to help developers build intelligent applications.\n\n"
	prompt += "Your role is to assist users with their development tasks, answer questions, and provide helpful suggestions.\n\n"
	prompt += "Be concise, practical, and focus on technical accuracy."

	return prompt
}

// buildLLMRequest builds LLM API request
func (r *Runtime) buildLLMRequest(msg *channels.Message, history []llm.Message, systemPrompt string) llm.Request {
	// Extract system prompt from message metadata if available
	extraPrompt := ""
	if metadata, ok := msg.Metadata["system_prompt"]; ok {
		extraPrompt = metadata.(string)
	}

	combinedSystemPrompt := systemPrompt
	if extraPrompt != "" {
		combinedSystemPrompt += "\n\n" + extraPrompt
	}

	return llm.Request{
		SystemPrompt: combinedSystemPrompt,
		Messages:     append(history, llm.Message{
			Role:    "user",
			Content: msg.Content,
		}),
		MaxTokens:    200000, // Default: 200K
		Temperature:  0.7,
		TopP:         5,
		Stream:       true, // Enable streaming
	}
}

// extractResponse extracts and streams LLM response
func (r *Runtime) extractResponse(llmResp *llm.Response) (*llm.StreamedResponse, error) {
	if llmResp == nil {
		return nil, fmt.Errorf("empty LLM response")
	}

	// Handle streaming vs non-streaming
	switch resp := llmResp.(type) {
	case *llm.StreamedResponse:
		// Streaming response - return as-is
		return resp.(*llm.StreamedResponse), nil

	case *llm.Response:
		// Non-streaming - convert to streamed
		single := resp.(*llm.Response)
		return &llm.StreamedResponse{
			Text:       single.Text,
			FinishReason: single.FinishReason,
			Usage:      single.Usage,
		}, nil

	default:
		return nil, fmt.Errorf("unknown response type: %T", resp)
	}
}

// eventLoop handles incoming events from event bus
func (r *Runtime) eventLoop() {
	defer r.wg.Done()

	for {
		select {
		case <-r.ctx.Done():
			return
		}
	}
}

// SetMessageHandler sets message handler for incoming messages
func (r *Runtime) SetMessageHandler(handler func(ctx context.Context, channel channels.Channel, msg *channels.Message) (string, error)) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// TODO: Register handler with session manager
	// For now, just log incoming messages
	go func() {
		for msg := range r.sessionMgr.GetAllSessions() {
			if handler != nil {
				ctx, cancel := context.WithTimeout(r.ctx, 30*time.Second)
				defer cancel()

				// Simulate incoming message for each session
				_, _ = handler(ctx, msg.Channel, &channels.Message{
					ID:       msg.ID,
					Channel:  msg.Channel,
					From:     r.llm.Provider(),
					To:       "all",
					Content:  fmt.Sprintf("Session update for %s", msg.SessionID),
					Type:     channels.MessageTypeText,
					Timestamp: time.Now(),
				})
			}
		}

		time.Sleep(5 * time.Second)
	}()
}

// GetStats returns runtime statistics
func (r *Runtime) GetStats() *RuntimeStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sessions := r.sessionMgr.GetAll()

	stats := &RuntimeStats{
		Running:     r.running,
		LLMProvider: r.llm.Provider(),
		LLMModel:      r.llm.Model(),
		Sessions:      len(sessions),
		Uptime:        time.Since(r.ctx).String(),
	}

	return stats
}

// RuntimeStats contains runtime statistics
type RuntimeStats struct {
	Running    bool      `json:"running"`
	LLMProvider string    `json:"llm_provider"`
	LLMModel   string    `json:"llm_model"`
	Sessions   int       `json:"sessions"`
	Uptime     string    `json:"uptime"`
}

// Status returns current status
func (r *Runtime) Status() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.running {
		return "running"
	}
	return "stopped"
}
