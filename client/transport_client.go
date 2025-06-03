package client

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// TransportMCPClient implements the MCPClient interface using the Transport abstraction.
// It can work with any transport implementation (stdio, in-memory, etc.)
type TransportMCPClient struct {
	transport    Transport
	capabilities mcp.ServerCapabilities
}

// NewTransportMCPClient creates a new MCP client using the specified transport
func NewTransportMCPClient(transport Transport) *TransportMCPClient {
	return &TransportMCPClient{
		transport: transport,
	}
}

// NewMCPClient creates a new MCP client with the specified transport type and configuration
func NewMCPClient(transportType string, config TransportConfig) (MCPClient, error) {
	transport, err := NewTransport(transportType, config)
	if err != nil {
		return nil, err
	}

	return NewTransportMCPClient(transport), nil
}

// Initialize sends the initial connection request to the server
func (c *TransportMCPClient) Initialize(
	ctx context.Context,
	request mcp.InitializeRequest,
) (*mcp.InitializeResult, error) {
	// Create params structure that ensures Capabilities is always included in JSON
	params := struct {
		ProtocolVersion string                 `json:"protocolVersion"`
		ClientInfo      mcp.Implementation     `json:"clientInfo"`
		Capabilities    mcp.ClientCapabilities `json:"capabilities"`
	}{
		ProtocolVersion: request.Params.ProtocolVersion,
		ClientInfo:      request.Params.ClientInfo,
		Capabilities:    request.Params.Capabilities, // Will be empty struct if not set
	}

	response, err := c.transport.SendRequest(ctx, "initialize", params)
	if err != nil {
		return nil, err
	}

	var result mcp.InitializeResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Store capabilities
	c.capabilities = result.Capabilities

	// Send initialized notification
	if err := c.transport.SendNotification(ctx, "notifications/initialized", nil); err != nil {
		return nil, fmt.Errorf("failed to send initialized notification: %w", err)
	}

	c.transport.SetInitialized(true)
	return &result, nil
}

// Ping checks if the server is alive
func (c *TransportMCPClient) Ping(ctx context.Context) error {
	_, err := c.transport.SendRequest(ctx, "ping", nil)
	return err
}

// ListResources requests a list of available resources from the server
func (c *TransportMCPClient) ListResources(
	ctx context.Context,
	request mcp.ListResourcesRequest,
) (*mcp.ListResourcesResult, error) {
	response, err := c.transport.SendRequest(ctx, "resources/list", request.Params)
	if err != nil {
		return nil, err
	}

	var result mcp.ListResourcesResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// ListResourceTemplates requests a list of available resource templates from the server
func (c *TransportMCPClient) ListResourceTemplates(
	ctx context.Context,
	request mcp.ListResourceTemplatesRequest,
) (*mcp.ListResourceTemplatesResult, error) {
	response, err := c.transport.SendRequest(ctx, "resources/templates/list", request.Params)
	if err != nil {
		return nil, err
	}

	var result mcp.ListResourceTemplatesResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// ReadResource reads a specific resource from the server
func (c *TransportMCPClient) ReadResource(
	ctx context.Context,
	request mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, error) {
	response, err := c.transport.SendRequest(ctx, "resources/read", request.Params)
	if err != nil {
		return nil, err
	}

	return mcp.ParseReadResourceResult(response)
}

// Subscribe requests notifications for changes to a specific resource
func (c *TransportMCPClient) Subscribe(ctx context.Context, request mcp.SubscribeRequest) error {
	_, err := c.transport.SendRequest(ctx, "resources/subscribe", request.Params)
	return err
}

// Unsubscribe cancels notifications for a specific resource
func (c *TransportMCPClient) Unsubscribe(ctx context.Context, request mcp.UnsubscribeRequest) error {
	_, err := c.transport.SendRequest(ctx, "resources/unsubscribe", request.Params)
	return err
}

// ListPrompts requests a list of available prompts from the server
func (c *TransportMCPClient) ListPrompts(
	ctx context.Context,
	request mcp.ListPromptsRequest,
) (*mcp.ListPromptsResult, error) {
	response, err := c.transport.SendRequest(ctx, "prompts/list", request.Params)
	if err != nil {
		return nil, err
	}

	var result mcp.ListPromptsResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// GetPrompt retrieves a specific prompt from the server
func (c *TransportMCPClient) GetPrompt(
	ctx context.Context,
	request mcp.GetPromptRequest,
) (*mcp.GetPromptResult, error) {
	response, err := c.transport.SendRequest(ctx, "prompts/get", request.Params)
	if err != nil {
		return nil, err
	}

	return mcp.ParseGetPromptResult(response)
}

// ListTools requests a list of available tools from the server
func (c *TransportMCPClient) ListTools(
	ctx context.Context,
	request mcp.ListToolsRequest,
) (*mcp.ListToolsResult, error) {
	response, err := c.transport.SendRequest(ctx, "tools/list", request.Params)
	if err != nil {
		return nil, err
	}

	var result mcp.ListToolsResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// CallTool invokes a specific tool on the server
func (c *TransportMCPClient) CallTool(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	response, err := c.transport.SendRequest(ctx, "tools/call", request.Params)
	if err != nil {
		return nil, err
	}

	return mcp.ParseCallToolResult(response)
}

// SetLevel sets the logging level for the server
func (c *TransportMCPClient) SetLevel(ctx context.Context, request mcp.SetLevelRequest) error {
	_, err := c.transport.SendRequest(ctx, "logging/setLevel", request.Params)
	return err
}

// Complete requests completion options for a given argument
func (c *TransportMCPClient) Complete(
	ctx context.Context,
	request mcp.CompleteRequest,
) (*mcp.CompleteResult, error) {
	response, err := c.transport.SendRequest(ctx, "completion/complete", request.Params)
	if err != nil {
		return nil, err
	}

	var result mcp.CompleteResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// Close client connection and cleanup resources
func (c *TransportMCPClient) Close() error {
	return c.transport.Close()
}

// OnNotification registers a handler for notifications
func (c *TransportMCPClient) OnNotification(handler func(notification mcp.JSONRPCNotification)) {
	c.transport.OnNotification(handler)
}
