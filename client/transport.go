package client

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
)

// Transport defines the communication interface for MCP clients.
// It abstracts the underlying communication mechanism (stdio, in-memory, etc.)
type Transport interface {
	// SendRequest sends a JSON-RPC request and waits for a response
	SendRequest(ctx context.Context, method string, params interface{}) (*json.RawMessage, error)

	// SendNotification sends a JSON-RPC notification (no response expected)
	SendNotification(ctx context.Context, method string, params interface{}) error

	// OnNotification registers a handler for incoming notifications
	OnNotification(handler func(notification mcp.JSONRPCNotification))

	// Close closes the transport and cleans up resources
	Close() error

	// IsInitialized returns true if the transport has been initialized
	IsInitialized() bool

	// SetInitialized marks the transport as initialized
	SetInitialized(initialized bool)
}

// TransportConfig holds configuration for different transport types
type TransportConfig struct {
	// Stdio configuration
	Command string
	Args    []string
	Env     []string

	// In-memory configuration
	Server interface{} // Will be mcp.Server interface when implemented
}

// NewTransport creates a new transport based on the configuration
func NewTransport(transportType string, config TransportConfig) (Transport, error) {
	switch transportType {
	case "stdio":
		transport, err := NewStdioTransport(config.Command, config.Env, config.Args...)
		if err != nil {
			return nil, err
		}
		return transport, nil
	case "memory":
		transport, err := NewMemoryTransport(config.Server)
		if err != nil {
			return nil, err
		}
		return transport, nil
	default:
		return nil, &UnsupportedTransportError{Type: transportType}
	}
}

// UnsupportedTransportError is returned when an unsupported transport type is requested
type UnsupportedTransportError struct {
	Type string
}

func (e *UnsupportedTransportError) Error() string {
	return "unsupported transport type: " + e.Type
}
