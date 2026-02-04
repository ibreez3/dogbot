package agent

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/openclaw/go-openclaw/internal/agent/llm"
	"github.com/openclaw/go-openclaw/internal/agent/session"
)

// Runtime represents Agent runtime
type Runtime struct {
	llm        llm.Client
	sessionMgr *session.Manager
	running    bool
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// Config represents Agent runtime configuration
type Config struct {
	LLMProvider  string        `mapstructure:"llm_provider"`
	LLMModel     string        `mapstructure:"llm_model"`
	MaxTokens    int           `mapstructure:"max_tokens"`
	Temperature  float64       `mapstructure:"temperature"`
	TopP         int           `mapstructure:"top_p"`
	Timeout      time.Duration `mapstructure:"timeout"`
	SystemPrompt string        `mapstructure:"system_prompt"`
	ToolsEnabled bool          `mapstructure:"tools_enabled"`
	APIKey       string        `mapstructure:"api_key"`
}

// DefaultConfig returns default Agent configuration
func DefaultConfig() *Config {
	return &Config{
		LLMProvider:  "anthropic",
		LLMModel:     "claude-3-5-sonnet-20241022",
		MaxTokens:    200000,
		Temperature:  0.7,
		TopP:         5,
		Timeout:      30 * time.Second,
		SystemPrompt: "You are OpenClaw, an AI agent platform built to help developers build intelligent applications.\n\nYour role is to assist users with their development tasks, answer questions, and provide helpful suggestions.\n\nBe concise, practical, and focus on technical accuracy.",
		ToolsEnabled: false,
		APIKey:       "",
	}
}

// NewRuntime creates a new Agent runtime
func NewRuntime(config *Config) (*Runtime, error) {
	// Create LLM client
	var llmClient llm.Client
	var err error

	switch config.LLMProvider {
	case "anthropic":
		llmClient, err = llm.NewAnthropicClient(config.APIKey, config.LLMModel, config.Timeout)
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", config.LLMProvider)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client: %w", err)
	}

	// Create session manager
	sessionMgr := session.NewManager()

	ctx, cancel := context.WithCancel(context.Background())

	return &Runtime{
		llm:        llmClient,
		sessionMgr: sessionMgr,
		running:    false,
		ctx:        ctx,
		cancel:     cancel,
		wg:         sync.WaitGroup{},
	}, nil
}

// Start starts Agent runtime
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

	r.wg.Add(1)
	go r.eventLoop()

	log.Printf("✅ Agent runtime started")

	return nil
}

// Stop stops Agent runtime
func (r *Runtime) Stop(ctx context.Context) error {
	log.Println("🛑 Stopping Agent runtime...")

	r.cancel()

	r.wg.Wait()

	log.Printf("✅ Agent runtime stopped")

	return nil
}

// ProcessMessage processes a message and returns LLM response
func (r *Runtime) ProcessMessage(ctx context.Context, channelID string, msg string) (string, error) {
	// Get or create session for this channel
	sess, err := r.sessionMgr.GetOrCreate(channelID)
	if err != nil {
		return "", fmt.Errorf("failed to get session: %w", err)
	}

	// Build message history for context
	history := r.buildMessageHistory(sess)

	// Build user prompt
	systemPrompt := r.buildSystemPrompt()

	// Prepare request
	llmReq := r.buildLLMRequest(msg, history, systemPrompt)

	// Call LLM
	llmResp, err := r.llm.SendMessage(ctx, &llmReq)
	if err != nil {
		return "", fmt.Errorf("LLM call failed: %w", err)
	}

	// Extract and stream response
	response, err := r.extractResponse(llmResp)
	if err != nil {
		return "", fmt.Errorf("response extraction failed: %w", err)
	}

	// Update session history
	sess.Messages = append(sess.Messages, &session.Message{
		Role:      "assistant",
		Content:   response.Text,
		Timestamp: time.Now(),
	})

	return response.Text, nil
}

// buildMessageHistory builds message history for LLM context
func (r *Runtime) buildMessageHistory(session *session.Session) []llm.Message {
	// Get recent messages (last 10)
	recent := session.Messages
	if len(recent) == 0 {
		return make([]llm.Message, 0)
	}

	// Convert to LLM message format
	history := make([]llm.Message, 0, len(recent))
	for i, msg := range recent {
		// LLM format: Role (user/assistant), Content
		role := "user"
		if msg.Role == "assistant" {
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
func (r *Runtime) buildLLMRequest(msg string, history []llm.Message, systemPrompt string) llm.Request {
	return llm.Request{
		SystemPrompt: systemPrompt,
		Messages: append(history, llm.Message{
			Role:    "user",
			Content: msg,
		}),
		MaxTokens:   200000,
		Temperature: 0.7,
		TopP:        5,
		Stream:      false,
	}
}

// extractResponse extracts LLM response
func (r *Runtime) extractResponse(llmResp *llm.Response) (*llm.Response, error) {
	if llmResp == nil {
		return nil, fmt.Errorf("empty LLM response")
	}

	return llmResp, nil
}

// eventLoop handles incoming events
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
func (r *Runtime) SetMessageHandler(handler func(ctx context.Context, channelID string, msg string) (string, error)) {
	r.mu.Lock()
	defer r.mu.Unlock()

	go func() {
		for {
			sessions := r.sessionMgr.GetAll()
			if handler != nil {
				for _, session := range sessions {
					ctx, cancel := context.WithTimeout(r.ctx, 30*time.Second)
					defer cancel()

					// Simulate incoming message for each session
					_, _ = handler(ctx, session.ID, fmt.Sprintf("Session update for %s", session.ID))
				}
			}

			time.Sleep(5 * time.Second)
		}
	}()
}

// GetStats returns runtime statistics
func (r *Runtime) GetStats() *RuntimeStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sessions := r.sessionMgr.GetAll()

	stats := &RuntimeStats{
		Running:     r.running,
		LLMProvider: string(r.llm.Provider()),
		LLMModel:    r.llm.Model(),
		Sessions:    len(sessions),
		Uptime:      time.Since(ctxStartTime(r.ctx)).String(),
	}

	return stats
}

// ctxStartTime extracts start time from context
func ctxStartTime(ctx context.Context) time.Time {
	if deadline, ok := ctx.Deadline(); ok {
		return deadline.AddDate(0, 0, -1)
	}
	return time.Now()
}

// RuntimeStats contains runtime statistics
type RuntimeStats struct {
	Running     bool   `json:"running"`
	LLMProvider string `json:"llm_provider"`
	LLMModel    string `json:"llm_model"`
	Sessions    int    `json:"sessions"`
	Uptime      string `json:"uptime"`
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
