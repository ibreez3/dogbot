package tools

import (
	"context"
	"fmt"
	"log"
	"sync"
)

// Registry manages tool registration and lookup
type Registry struct {
	tools    map[string]*Tool
	handlers map[string]ToolHandler
	mu       sync.RWMutex
}

// ToolHandler handles tool execution
type ToolHandler func(ctx context.Context, params map[string]interface{}) (*ToolResult, error)

// Tool represents a callable tool
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
	Function    string                 `json:"function"`  // For LLM function calling
	Handler     ToolHandler `json:"-"`
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Name       string                 `json:"name"`
	Success    bool                     `json:"success"`
	Data       map[string]interface{} `json:"data"`
	Error      string                    `json:"error,omitempty"`
	ExecTime   float64                  `json:"exec_time"`
}

// NewRegistry creates a new tool registry
func NewRegistry() *Registry {
	return &Registry{
		tools:    make(map[string]*Tool),
		handlers: make(map[string]ToolHandler),
	}
}

// Register registers a new tool
func (r *Registry) Register(tool *Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if tool.Name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	if _, exists := r.tools[tool.Name]; exists {
		return fmt.Errorf("tool already registered: %s", tool.Name)
	}

	r.tools[tool.Name] = tool
	log.Printf("🔧 Tool registered: %s", tool.Name)

	return nil
}

// Unregister removes a tool
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[name]; !exists {
		return fmt.Errorf("tool not found: %s", name)
	}

	delete(r.tools, name)
	log.Printf("📝 Tool unregistered: %s", name)

	return nil
}

// Get returns a tool by name
func (r *Registry) Get(name string) (*Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, ok := r.tools[name]
	return tool, ok
}

// GetAll returns all registered tools
func (r *Registry) GetAll() []*Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]*Tool, 0, len(r.tools))
	i := 0
	for _, tool := range r.tools {
		tools[i] = tool
		i++
	}

	return tools
}

// Execute runs a tool by name
func (r *Registry) Execute(ctx context.Context, name string, params map[string]interface{}) (*ToolResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.tools[name]
	if !ok {
		return &ToolResult{
			Name:    name,
			Success: false,
			Error:   fmt.Sprintf("tool not found: %s", name),
		}, nil
	}

	handler, ok := r.handlers[name]
	if !ok {
		return &ToolResult{
			Name:    name,
			Success: false,
			Error:   fmt.Sprintf("handler not found: %s", name),
		}, nil
	}

	// Execute handler with timeout
	result, err := handler(ctx, params)
	if err != nil {
		return &ToolResult{
			Name:    name,
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return result, nil
}

// RegisterHandler registers a tool handler
func (r *Registry) RegisterHandler(name string, handler ToolHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}

	r.handlers[name] = handler

	return nil
}

// Start starts the tool registry
func (r *Registry) Start(ctx context.Context) error {
	log.Printf("🔧 Starting tool registry...")
	// For now, just log the number of tools
	log.Printf("✅ Tool registry started (tools=%d)", len(r.tools))

	return nil
}

// Stop stops the tool registry
func (r *Registry) Stop(ctx context.Context) error {
	log.Printf("🛑 Stopping tool registry...")

	// For now, just log
	log.Printf("✅ Tool registry stopped")

	return nil
}
