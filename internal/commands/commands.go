package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/openclaw/go-openclaw/internal/events"
	"github.com/openclaw/go-openclaw/internal/protocol"
	"go.uber.org/zap"
)

// CommandContext provides context for command execution
type CommandContext struct {
	ClientID  string
	SessionID string
	DeviceID  string
	Logger    *zap.Logger
	EventBus  *events.EventBus
	Gateway   interface{} // Avoid circular dependency
}

// CommandHandler handles specific commands
type CommandHandler func(ctx context.Context, cmdCtx *CommandContext, params json.RawMessage) (interface{}, error)

// Registry manages command handlers
type Registry struct {
	handlers map[string]CommandHandler
	logger   *zap.Logger
}

// NewRegistry creates a new command registry
func NewRegistry(logger *zap.Logger) *Registry {
	return &Registry{
		handlers: make(map[string]CommandHandler),
		logger:   logger,
	}
}

// Register registers a command handler
func (r *Registry) Register(method string, handler CommandHandler) {
	r.handlers[method] = handler
	r.logger.Debug("Command registered", zap.String("method", method))
}

// Handle handles a command request
func (r *Registry) Handle(ctx context.Context, cmdCtx *CommandContext, method string, params json.RawMessage) (*protocol.ProtocolMessage, error) {
	handler, ok := r.handlers[method]
	if !ok {
		return nil, fmt.Errorf("unknown method: %s", method)
	}

	payload, err := handler(ctx, cmdCtx, params)
	if err != nil {
		return nil, err
	}

	return protocol.NewResponse(cmdCtx.SessionID, true, payload, ""), nil
}

// SetupDefaultHandlers registers default command handlers
func (r *Registry) SetupDefaultHandlers(gateway interface{}, eventBus *events.EventBus, logger *zap.Logger) {
	cmdCtx := &CommandContext{
		Logger:   logger,
		EventBus: eventBus,
		Gateway:  gateway,
	}

	// Register health command
	r.Register("health", r.handleHealth(cmdCtx))
	r.Register("ping", r.handlePing(cmdCtx))
	r.Register("agent", r.handleAgent(cmdCtx))
	r.Register("workspace", r.handleWorkspace(cmdCtx))
	r.Register("node", r.handleNode(cmdCtx))
}

// handleHealth handles health check commands
func (r *Registry) handleHealth(cmdCtx *CommandContext) CommandHandler {
	return func(ctx context.Context, cc *CommandContext, params json.RawMessage) (interface{}, error) {
		var req protocol.HealthRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("invalid health request: %w", err)
		}

		// Get uptime (would need to be stored in gateway)
		uptime := int64(0) // TODO: implement

		response := &protocol.HealthResponse{
			Status: "ok",
			Uptime: fmt.Sprintf("%d", uptime),
			Checks: []protocol.CheckResult{
				{Name: "server", Status: "ok", Message: "Running"},
				{Name: "events", Status: "ok", Message: "Event system operational"},
			},
			Timestamp: time.Now().Unix(),
		}

		// Publish health event
		healthData := &events.HealthEventData{
			Component: "gateway",
			Status:    "ok",
			Message:   "Health check requested",
		}
		event, _ := events.NewEvent(string(events.EventHealth), "check", healthData, "gateway")
		cc.EventBus.PublishAsync(event)

		return response, nil
	}
}

// handlePing handles ping commands
func (r *Registry) handlePing(cmdCtx *CommandContext) CommandHandler {
	return func(ctx context.Context, cc *CommandContext, params json.RawMessage) (interface{}, error) {
		return map[string]any{
			"pong": true,
			"time": time.Now().Unix(),
		}, nil
	}
}

// handleAgent handles agent-related commands
func (r *Registry) handleAgent(cmdCtx *CommandContext) CommandHandler {
	return func(ctx context.Context, cc *CommandContext, params json.RawMessage) (interface{}, error) {
		var req protocol.AgentRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("invalid agent request: %w", err)
		}

		switch req.Action {
		case "list", "":
			return &protocol.AgentResponse{
				Status: "ok",
				Agents: []protocol.AgentInfo{},
			}, nil

		case "get":
			agentID := ""
			if req.ChannelID != "" {
				agentID = req.ChannelID
			}
			if agentID == "" {
				return nil, fmt.Errorf("agent_id is required for get action")
			}
			return &protocol.AgentResponse{
				Status: "ok",
				Data:   map[string]interface{}{"agent_id": agentID},
			}, nil

		case "status":
			return &protocol.AgentResponse{
				Status:  "ok",
				Message: "Agent operational",
			}, nil

		default:
			return nil, fmt.Errorf("unsupported action: %s", req.Action)
		}
	}
}

// handleWorkspace handles workspace commands
func (r *Registry) handleWorkspace(cmdCtx *CommandContext) CommandHandler {
	return func(ctx context.Context, cc *CommandContext, params json.RawMessage) (interface{}, error) {
		var req protocol.WorkspaceRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("invalid workspace request: %w", err)
		}

		switch req.Action {
		case "list", "":
			return &protocol.WorkspaceResponse{
				Status:     "ok",
				Workspaces: []protocol.WorkspaceInfo{},
			}, nil

		case "get":
			return &protocol.WorkspaceResponse{
				Status: "ok",
				Workspace: &protocol.WorkspaceInfo{
					ID:   "default",
					Name: "Default Workspace",
				},
			}, nil

		case "switch":
			return &protocol.WorkspaceResponse{
				Status:  "ok",
				Message: fmt.Sprintf("Switched to workspace: %s", req.Workspace),
			}, nil

		default:
			return nil, fmt.Errorf("unsupported action: %s", req.Action)
		}
	}
}

// handleNode handles node commands
func (r *Registry) handleNode(cmdCtx *CommandContext) CommandHandler {
	return func(ctx context.Context, cc *CommandContext, params json.RawMessage) (interface{}, error) {
		var req protocol.NodeRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("invalid node request: %w", err)
		}

		switch req.Action {
		case "list", "":
			return &protocol.NodeResponse{
				Status: "ok",
				Nodes:  []protocol.NodeInfo{},
			}, nil

		case "get":
			nodeID := ""
			if req.ChannelID != "" {
				nodeID = req.ChannelID
			}
			if nodeID == "" {
				return nil, fmt.Errorf("node_id is required for get action")
			}
			return &protocol.NodeResponse{
				Status: "ok",
				Node:   &protocol.NodeInfo{ID: nodeID},
			}, nil

		case "notify":
			// Publish notification event
			eventData := &events.AgentEventData{
				Action:  "notify",
				NodeID:  req.ChannelID,
				Details: make(map[string]any),
			}
			event, _ := events.NewEvent(string(events.EventNode), "notify", eventData, "gateway")
			cc.EventBus.PublishAsync(event)

			return &protocol.NodeResponse{
				Status:  "ok",
				Message: "Notification sent",
			}, nil

		default:
			return nil, fmt.Errorf("unsupported action: %s", req.Action)
		}
	}
}
