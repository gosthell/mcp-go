package client

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"

	"github.com/mark3labs/mcp-go/mcp"
)

// RPCResponse represents a JSON-RPC response
type RPCResponse struct {
	Error    *string          `json:"error,omitempty"`
	Response *json.RawMessage `json:"result,omitempty"`
}

// StdioTransport implements the Transport interface using stdio communication.
// It launches a subprocess and communicates with it via standard input/output streams
// using JSON-RPC messages.
type StdioTransport struct {
	cmd           *exec.Cmd
	stdin         io.WriteCloser
	stdout        *bufio.Reader
	stderr        *bufio.Reader
	requestID     atomic.Int64
	responses     map[int64]chan RPCResponse
	mu            sync.RWMutex
	done          chan struct{}
	initialized   atomic.Bool
	notifications []func(mcp.JSONRPCNotification)
	notifyMu      sync.RWMutex
}

// NewStdioTransport creates a new stdio-based transport that communicates with a subprocess.
func NewStdioTransport(command string, env []string, args ...string) (*StdioTransport, error) {
	cmd := exec.Command(command, args...)

	mergedEnv := os.Environ()
	mergedEnv = append(mergedEnv, env...)
	cmd.Env = mergedEnv

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	transport := &StdioTransport{
		cmd:       cmd,
		stdin:     stdin,
		stdout:    bufio.NewReader(stdout),
		stderr:    bufio.NewReader(stderr),
		responses: make(map[int64]chan RPCResponse),
		done:      make(chan struct{}),
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	// Start reading stderr in a goroutine
	go func() {
		for {
			select {
			case <-transport.done:
				return
			default:
				line, err := transport.stderr.ReadString('\n')
				if err != nil {
					if err != io.EOF {
						fmt.Printf("Error reading stderr: %v\n", err)
					}
					return
				}
				fmt.Printf("Server error: %s", line)
			}
		}
	}()

	// Start reading responses in a goroutine
	ready := make(chan struct{})
	go func() {
		close(ready)
		transport.readResponses()
	}()
	<-ready

	return transport, nil
}

// SendRequest sends a JSON-RPC request and waits for a response
func (t *StdioTransport) SendRequest(ctx context.Context, method string, params interface{}) (*json.RawMessage, error) {
	if !t.initialized.Load() && method != "initialize" {
		return nil, fmt.Errorf("transport not initialized")
	}

	id := t.requestID.Add(1)

	request := mcp.JSONRPCRequest{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      id,
		Request: mcp.Request{
			Method: method,
		},
		Params: params,
	}

	responseChan := make(chan RPCResponse, 1)
	t.mu.Lock()
	t.responses[id] = responseChan
	t.mu.Unlock()

	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	requestBytes = append(requestBytes, '\n')

	if _, err := t.stdin.Write(requestBytes); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	select {
	case <-ctx.Done():
		t.mu.Lock()
		delete(t.responses, id)
		t.mu.Unlock()
		return nil, ctx.Err()
	case response := <-responseChan:
		if response.Error != nil {
			return nil, errors.New(*response.Error)
		}
		return response.Response, nil
	}
}

// SendNotification sends a JSON-RPC notification
func (t *StdioTransport) SendNotification(ctx context.Context, method string, params interface{}) error {
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

	notificationBytes, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}
	notificationBytes = append(notificationBytes, '\n')

	if _, err := t.stdin.Write(notificationBytes); err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}

	return nil
}

// OnNotification registers a handler for incoming notifications
func (t *StdioTransport) OnNotification(handler func(notification mcp.JSONRPCNotification)) {
	t.notifyMu.Lock()
	defer t.notifyMu.Unlock()
	t.notifications = append(t.notifications, handler)
}

// Close shuts down the stdio transport
func (t *StdioTransport) Close() error {
	close(t.done)
	if err := t.stdin.Close(); err != nil {
		return fmt.Errorf("failed to close stdin: %w", err)
	}
	return t.cmd.Wait()
}

// IsInitialized returns true if the transport has been initialized
func (t *StdioTransport) IsInitialized() bool {
	return t.initialized.Load()
}

// SetInitialized marks the transport as initialized
func (t *StdioTransport) SetInitialized(initialized bool) {
	t.initialized.Store(initialized)
}

// readResponses continuously reads and processes responses from the server's stdout
func (t *StdioTransport) readResponses() {
	for {
		select {
		case <-t.done:
			return
		default:
			line, err := t.stdout.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					fmt.Printf("Error reading response: %v\n", err)
				}
				return
			}

			var baseMessage struct {
				JSONRPC string          `json:"jsonrpc"`
				ID      *int64          `json:"id,omitempty"`
				Method  string          `json:"method,omitempty"`
				Result  json.RawMessage `json:"result,omitempty"`
				Error   *struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				} `json:"error,omitempty"`
			}

			if err := json.Unmarshal([]byte(line), &baseMessage); err != nil {
				continue
			}

			// Handle notification
			if baseMessage.ID == nil {
				var notification mcp.JSONRPCNotification
				if err := json.Unmarshal([]byte(line), &notification); err != nil {
					continue
				}
				t.notifyMu.RLock()
				for _, handler := range t.notifications {
					go handler(notification)
				}
				t.notifyMu.RUnlock()
				continue
			}

			t.mu.RLock()
			ch, ok := t.responses[*baseMessage.ID]
			t.mu.RUnlock()

			if ok {
				if baseMessage.Error != nil {
					ch <- RPCResponse{
						Error: &baseMessage.Error.Message,
					}
				} else {
					ch <- RPCResponse{
						Response: &baseMessage.Result,
					}
				}
				t.mu.Lock()
				delete(t.responses, *baseMessage.ID)
				t.mu.Unlock()
			}
		}
	}
}
