package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/openclaw/go-openclaw/internal/agent/llm"
)

// Executor handles tool execution
type Executor struct {
	llm       llm.Client
	registry  *Registry
	running   bool
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	startTime time.Time
}

// NewExecutor creates a new tool executor
func NewExecutor(llmClient llm.Client, registry *Registry) *Executor {
	return &Executor{
		llm:     llmClient,
		registry: registry,
		running:  false,
		ctx:      context.Background(),
		cancel:   func() {},
		wg:       sync.WaitGroup{},
	}
}

// Start starts the executor
func (e *Executor) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return fmt.Errorf("executor is already running")
	}

	e.ctx, e.cancel = context.WithCancel(ctx)
	e.running = true
	e.startTime = time.Now()
	e.mu.Unlock()

	log.Printf("🔧 Starting tool executor...")

	// Start registry
	if err := e.registry.Start(e.ctx); err != nil {
		return fmt.Errorf("failed to start tool registry: %w", err)
	}

	log.Printf("✅ Tool executor started")

	return nil
}

// Stop stops the executor
func (e *Executor) Stop(ctx context.Context) error {
	e.mu.Lock()
	e.running = false
	e.mu.Unlock()

	log.Printf("🛑 Stopping tool executor...")

	e.cancel()

	// Stop registry
	_ = e.registry.Stop(context.Background())

	e.wg.Wait()

	log.Printf("✅ Tool executor stopped")

	return nil
}

// Execute executes a tool by name
func (e *Executor) Execute(ctx context.Context, toolName string, params map[string]interface{}) (*ToolResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Get tool from registry
	tool, ok := e.registry.Get(toolName)
	if !ok {
		return &ToolResult{
			Name:    toolName,
			Success: false,
			Error:   fmt.Sprintf("tool not found: %s", toolName),
		}, nil
	}

	startTime := time.Now()

	// Execute tool handler
	result, err := tool.Handler(ctx, params)
	if err != nil {
		return &ToolResult{
			Name:    toolName,
			Success: false,
			Error:   err.Error(),
			Data:     nil,
		}, nil
	}

	execTime := time.Since(startTime).Seconds()

	// Log execution
	log.Printf("🔧 Tool executed: name=%s success=%v exec_time=%.2fs", toolName, result.Success, execTime)

	// Extract result data
	var data map[string]interface{}
	if result.Success {
		if result.Data != nil {
			data = result.Data
		}
	}

	return &ToolResult{
		Name:     toolName,
		Success:  result.Success,
		Data:     data,
		ExecTime: execTime,
		Error:    "",
	}, nil
}

// CallTool calls a tool via LLM
func (e *Executor) CallTool(ctx context.Context, toolName string, params map[string]interface{}) (*ToolResult, error) {
	// First, try direct tool execution
	result, err := e.Execute(ctx, toolName, params)
	if err == nil && result.Success {
		return result, nil
	}

	// If direct execution failed, try LLM function calling
	startTime := time.Now()
	
	// Build LLM request with tool call
	toolRequest := &llm.Request{
		Messages: []llm.Message{
			{
				Role:    "user",
				Content: fmt.Sprintf("Call tool: %s with params: %v", toolName, params),
			},
		},
	}

	// Send to LLM
	llmResp, err := e.llm.SendMessage(ctx, toolRequest)
	if err != nil {
		return nil, err
	}

	// Parse tool response from LLM
	var toolResult map[string]interface{}
	if llmResp != nil && llmResp.Text != "" {
		// Extract JSON response
		err := json.Unmarshal([]byte(llmResp.Text), &toolResult)
		if err != nil {
			log.Printf("⚠️ Failed to parse tool result: %v", err)
		}

		if llmResp.Usage != nil {
			toolResult["usage"] = map[string]interface{}{
				"input_tokens":  llmResp.Usage.InputTokens,
				"output_tokens": llmResp.Usage.OutputTokens,
				"total_tokens":  llmResp.Usage.TotalTokens,
			}
		}
	}

	execTime := time.Since(startTime).Seconds()

	return &ToolResult{
		Name:     toolName,
		Success:  true,
		Data:     toolResult,
		ExecTime: execTime,
		Error:    "",
	}, nil
}

// IsRunning checks if executor is running
func (e *Executor) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// GetStats returns executor statistics
func (e *Executor) GetStats() *ExecutorStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Count tools
	toolCount := e.registry.GetAll()

	var uptime string
	if !e.startTime.IsZero() {
		uptime = time.Since(e.startTime).String()
	}

	return &ExecutorStats{
		Running:   e.running,
		ToolCount: len(toolCount),
		Uptime:    uptime,
	}
}

// ExecutorStats contains executor statistics
type ExecutorStats struct {
	Running    bool     `json:"running"`
	ToolCount int       `json:"tool_count"`
	Uptime     string    `json:"uptime"`
}
