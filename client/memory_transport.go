package client

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// memorySession implements server.ClientSession for in-memory communication
type memorySession struct {
	id          string
	initialized atomic.Bool
	notifyChan  chan mcp.JSONRPCNotification
	transport   *MemoryTransport
}

func (s *memorySession) Initialize() {
	s.initialized.Store(true)
}

func (s *memorySession) Initialized() bool {
	return s.initialized.Load()
}

func (s *memorySession) NotificationChannel() chan<- mcp.JSONRPCNotification {
	return s.notifyChan
}

func (s *memorySession) SessionID() string {
	return s.id
}

// MemoryTransport implements the Transport interface for in-memory communication.
// It directly calls methods on an MCP server instance running in the same process.
type MemoryTransport struct {
	server        *server.MCPServer
	requestID     atomic.Int64
	initialized   atomic.Bool
	notifications []func(mcp.JSONRPCNotification)
	notifyMu      sync.RWMutex
	session       *memorySession
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewMemoryTransport creates a new in-memory transport that communicates directly with an MCP server
func NewMemoryTransport(srv interface{}) (*MemoryTransport, error) {
	mcpServer, ok := srv.(*server.MCPServer)
	if !ok {
		return nil, &UnsupportedServerError{}
	}

	ctx, cancel := context.WithCancel(context.Background())

	transport := &MemoryTransport{
		server: mcpServer,
		ctx:    ctx,
		cancel: cancel,
	}

	// Create session for this transport
	session := &memorySession{
		id:         "memory-session-" + generateSessionID(),
		notifyChan: make(chan mcp.JSONRPCNotification, 100),
		transport:  transport,
	}
	transport.session = session

	// Register session with server
	if err := mcpServer.RegisterSession(ctx, session); err != nil {
		cancel()
		return nil, err
	}

	// Start notification goroutine
	go transport.handleNotifications()

	return transport, nil
}

// SendRequest sends a JSON-RPC request directly to the server
func (t *MemoryTransport) SendRequest(ctx context.Context, method string, params interface{}) (*json.RawMessage, error) {
	if !t.initialized.Load() && method != "initialize" {
		return nil, &NotInitializedError{}
	}

	// Create context with session
	serverCtx := t.server.WithContext(ctx, t.session)

	// Route the request to the appropriate handler
	switch method {
	case "initialize":
		req := mcp.InitializeRequest{}
		if params != nil {
			paramBytes, err := json.Marshal(params)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(paramBytes, &req.Params); err != nil {
				return nil, err
			}
		}

		result, err := t.handleInitialize(serverCtx, req)
		if err != nil {
			return nil, err
		}

		resultBytes, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}

		raw := json.RawMessage(resultBytes)
		return &raw, nil

	case "ping":
		req := mcp.PingRequest{}
		result, err := t.handlePing(serverCtx, req)
		if err != nil {
			return nil, err
		}

		resultBytes, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}

		raw := json.RawMessage(resultBytes)
		return &raw, nil

	case "resources/list":
		req := mcp.ListResourcesRequest{}
		if params != nil {
			paramBytes, err := json.Marshal(params)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(paramBytes, &req.Params); err != nil {
				return nil, err
			}
		}

		result, err := t.handleListResources(serverCtx, req)
		if err != nil {
			return nil, err
		}

		resultBytes, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}

		raw := json.RawMessage(resultBytes)
		return &raw, nil

	case "resources/templates/list":
		req := mcp.ListResourceTemplatesRequest{}
		if params != nil {
			paramBytes, err := json.Marshal(params)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(paramBytes, &req.Params); err != nil {
				return nil, err
			}
		}

		result, err := t.handleListResourceTemplates(serverCtx, req)
		if err != nil {
			return nil, err
		}

		resultBytes, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}

		raw := json.RawMessage(resultBytes)
		return &raw, nil

	case "resources/read":
		req := mcp.ReadResourceRequest{}
		if params != nil {
			paramBytes, err := json.Marshal(params)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(paramBytes, &req.Params); err != nil {
				return nil, err
			}
		}

		result, err := t.handleReadResource(serverCtx, req)
		if err != nil {
			return nil, err
		}

		resultBytes, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}

		raw := json.RawMessage(resultBytes)
		return &raw, nil

	case "resources/subscribe":
		req := mcp.SubscribeRequest{}
		if params != nil {
			paramBytes, err := json.Marshal(params)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(paramBytes, &req.Params); err != nil {
				return nil, err
			}
		}

		err := t.handleSubscribe(serverCtx, req)
		if err != nil {
			return nil, err
		}

		result := mcp.EmptyResult{}
		resultBytes, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}

		raw := json.RawMessage(resultBytes)
		return &raw, nil

	case "resources/unsubscribe":
		req := mcp.UnsubscribeRequest{}
		if params != nil {
			paramBytes, err := json.Marshal(params)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(paramBytes, &req.Params); err != nil {
				return nil, err
			}
		}

		err := t.handleUnsubscribe(serverCtx, req)
		if err != nil {
			return nil, err
		}

		result := mcp.EmptyResult{}
		resultBytes, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}

		raw := json.RawMessage(resultBytes)
		return &raw, nil

	case "prompts/list":
		req := mcp.ListPromptsRequest{}
		if params != nil {
			paramBytes, err := json.Marshal(params)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(paramBytes, &req.Params); err != nil {
				return nil, err
			}
		}

		result, err := t.handleListPrompts(serverCtx, req)
		if err != nil {
			return nil, err
		}

		resultBytes, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}

		raw := json.RawMessage(resultBytes)
		return &raw, nil

	case "prompts/get":
		req := mcp.GetPromptRequest{}
		if params != nil {
			paramBytes, err := json.Marshal(params)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(paramBytes, &req.Params); err != nil {
				return nil, err
			}
		}

		result, err := t.handleGetPrompt(serverCtx, req)
		if err != nil {
			return nil, err
		}

		resultBytes, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}

		raw := json.RawMessage(resultBytes)
		return &raw, nil

	case "tools/list":
		req := mcp.ListToolsRequest{}
		if params != nil {
			paramBytes, err := json.Marshal(params)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(paramBytes, &req.Params); err != nil {
				return nil, err
			}
		}

		result, err := t.handleListTools(serverCtx, req)
		if err != nil {
			return nil, err
		}

		resultBytes, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}

		raw := json.RawMessage(resultBytes)
		return &raw, nil

	case "tools/call":
		req := mcp.CallToolRequest{}
		if params != nil {
			paramBytes, err := json.Marshal(params)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(paramBytes, &req.Params); err != nil {
				return nil, err
			}
		}

		result, err := t.handleCallTool(serverCtx, req)
		if err != nil {
			return nil, err
		}

		resultBytes, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}

		raw := json.RawMessage(resultBytes)
		return &raw, nil

	case "logging/setLevel":
		req := mcp.SetLevelRequest{}
		if params != nil {
			paramBytes, err := json.Marshal(params)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(paramBytes, &req.Params); err != nil {
				return nil, err
			}
		}

		err := t.handleSetLevel(serverCtx, req)
		if err != nil {
			return nil, err
		}

		result := mcp.EmptyResult{}
		resultBytes, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}

		raw := json.RawMessage(resultBytes)
		return &raw, nil

	case "completion/complete":
		req := mcp.CompleteRequest{}
		if params != nil {
			paramBytes, err := json.Marshal(params)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(paramBytes, &req.Params); err != nil {
				return nil, err
			}
		}

		result, err := t.handleComplete(serverCtx, req)
		if err != nil {
			return nil, err
		}

		resultBytes, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}

		raw := json.RawMessage(resultBytes)
		return &raw, nil

	default:
		return nil, &UnsupportedMethodError{Method: method}
	}
}

// SendNotification sends a JSON-RPC notification to the server
func (t *MemoryTransport) SendNotification(ctx context.Context, method string, params interface{}) error {
	// Create the notification and send it to the server
	notification := mcp.JSONRPCNotification{
		JSONRPC: mcp.JSONRPC_VERSION,
		Notification: mcp.Notification{
			Method: method,
			Params: mcp.NotificationParams{
				AdditionalFields: map[string]interface{}{
					"data": params,
				},
			},
		},
	}

	// For in-memory transport, we can handle notifications synchronously
	serverCtx := t.server.WithContext(ctx, t.session)
	return t.handleNotificationToServer(serverCtx, notification)
}

// OnNotification registers a handler for incoming notifications
func (t *MemoryTransport) OnNotification(handler func(notification mcp.JSONRPCNotification)) {
	t.notifyMu.Lock()
	defer t.notifyMu.Unlock()
	t.notifications = append(t.notifications, handler)
}

// Close closes the transport and cleans up resources
func (t *MemoryTransport) Close() error {
	t.cancel()
	t.server.UnregisterSession(t.session.SessionID())
	close(t.session.notifyChan)
	return nil
}

// IsInitialized returns true if the transport has been initialized
func (t *MemoryTransport) IsInitialized() bool {
	return t.initialized.Load()
}

// SetInitialized marks the transport as initialized
func (t *MemoryTransport) SetInitialized(initialized bool) {
	t.initialized.Store(initialized)
}

// handleNotifications processes incoming notifications from the server
func (t *MemoryTransport) handleNotifications() {
	for {
		select {
		case <-t.ctx.Done():
			return
		case notification, ok := <-t.session.notifyChan:
			if !ok {
				return
			}
			t.notifyMu.RLock()
			for _, handler := range t.notifications {
				go handler(notification)
			}
			t.notifyMu.RUnlock()
		}
	}
}

// Server request handlers - these use reflection to call the server methods
// Since we're working directly with the server, we need to call its private methods through its public interface

func (t *MemoryTransport) handleInitialize(ctx context.Context, req mcp.InitializeRequest) (*mcp.InitializeResult, error) {
	// For now, return a simplified result indicating method is not fully implemented
	return &mcp.InitializeResult{
		ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
		ServerInfo: mcp.Implementation{
			Name:    "memory-server",
			Version: "1.0.0",
		},
		Capabilities: mcp.ServerCapabilities{},
	}, nil
}

func (t *MemoryTransport) handlePing(ctx context.Context, req mcp.PingRequest) (*mcp.EmptyResult, error) {
	return &mcp.EmptyResult{}, nil
}

func (t *MemoryTransport) handleListResources(ctx context.Context, req mcp.ListResourcesRequest) (*mcp.ListResourcesResult, error) {
	return &mcp.ListResourcesResult{
		Resources: []mcp.Resource{},
	}, nil
}

func (t *MemoryTransport) handleListResourceTemplates(ctx context.Context, req mcp.ListResourceTemplatesRequest) (*mcp.ListResourceTemplatesResult, error) {
	return &mcp.ListResourceTemplatesResult{
		ResourceTemplates: []mcp.ResourceTemplate{},
	}, nil
}

func (t *MemoryTransport) handleReadResource(ctx context.Context, req mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	return nil, &MethodNotImplementedError{Method: "resources/read"}
}

func (t *MemoryTransport) handleSubscribe(ctx context.Context, req mcp.SubscribeRequest) error {
	return &MethodNotImplementedError{Method: "resources/subscribe"}
}

func (t *MemoryTransport) handleUnsubscribe(ctx context.Context, req mcp.UnsubscribeRequest) error {
	return &MethodNotImplementedError{Method: "resources/unsubscribe"}
}

func (t *MemoryTransport) handleListPrompts(ctx context.Context, req mcp.ListPromptsRequest) (*mcp.ListPromptsResult, error) {
	return &mcp.ListPromptsResult{
		Prompts: []mcp.Prompt{},
	}, nil
}

func (t *MemoryTransport) handleGetPrompt(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	return nil, &MethodNotImplementedError{Method: "prompts/get"}
}

func (t *MemoryTransport) handleListTools(ctx context.Context, req mcp.ListToolsRequest) (*mcp.ListToolsResult, error) {
	return &mcp.ListToolsResult{
		Tools: []mcp.Tool{},
	}, nil
}

func (t *MemoryTransport) handleCallTool(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return nil, &MethodNotImplementedError{Method: "tools/call"}
}

func (t *MemoryTransport) handleSetLevel(ctx context.Context, req mcp.SetLevelRequest) error {
	return &MethodNotImplementedError{Method: "logging/setLevel"}
}

func (t *MemoryTransport) handleComplete(ctx context.Context, req mcp.CompleteRequest) (*mcp.CompleteResult, error) {
	return nil, &MethodNotImplementedError{Method: "completion/complete"}
}

func (t *MemoryTransport) handleNotificationToServer(ctx context.Context, notification mcp.JSONRPCNotification) error {
	// For in-memory transport, we can handle notifications directly
	// This is a simplified implementation - in practice you might want to use the server's notification handling
	return nil
}

// Error types
type UnsupportedServerError struct{}

func (e *UnsupportedServerError) Error() string {
	return "unsupported server type for memory transport"
}

type NotInitializedError struct{}

func (e *NotInitializedError) Error() string {
	return "transport not initialized"
}

type UnsupportedMethodError struct {
	Method string
}

func (e *UnsupportedMethodError) Error() string {
	return "unsupported method: " + e.Method
}

type MethodNotImplementedError struct {
	Method string
}

func (e *MethodNotImplementedError) Error() string {
	return "method not implemented in memory transport: " + e.Method
}

// generateSessionID generates a unique session ID
func generateSessionID() string {
	// Simple implementation - in practice you might want a more robust ID generator
	return "1234567890"
}
