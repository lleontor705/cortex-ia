// Package mcpserve exposes the cortex-ia work-control surface to MCP hosts as a
// newline-delimited JSON-RPC 2.0 stdio server. Authority tokens stay in this
// process's memory and never leave it: results are scrubbed before they are
// returned to the host.
package mcpserve

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/lleontor705/cortex-ia/v2/internal/hostbridge"
)

// Version is reported in the initialize result; the CLI sets it before Serve.
var Version = "dev"

const (
	defaultProtocolVersion = "2025-06-18"
	maxLineBytes           = 8 << 20
	errParseError          = -32700
	errInvalidRequest      = -32600
	errMethodNotFound      = -32601
	errInvalidParams       = -32602
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// Serve runs the MCP server for host until in reaches EOF or ctx is cancelled.
// Heartbeats for every live claim are stopped on return.
func Serve(ctx context.Context, host hostbridge.Host, in io.Reader, out io.Writer) error {
	s, err := newSession(ctx, host)
	if err != nil {
		return err
	}
	return serveWith(ctx, s, in, out)
}

func serveWith(ctx context.Context, s *session, in io.Reader, out io.Writer) error {
	defer s.vault.stopAll()
	reader := bufio.NewReaderSize(in, 64<<10)
	var writeMu sync.Mutex
	write := func(resp *rpcResponse) error {
		raw, err := json.Marshal(resp)
		if err != nil {
			return err
		}
		writeMu.Lock()
		defer writeMu.Unlock()
		_, err = out.Write(append(raw, '\n'))
		return err
	}
	for {
		if ctx.Err() != nil {
			return nil
		}
		line, tooLong, readErr := readLine(reader)
		switch {
		case tooLong:
			if err := write(errorResponse(nil, errInvalidRequest, "message exceeds the 8 MiB line limit")); err != nil {
				return err
			}
		case len(bytes.TrimSpace(line)) > 0:
			if resp := s.handleLine(ctx, line); resp != nil {
				if err := write(resp); err != nil {
					return err
				}
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return readErr
		}
	}
}

// readLine returns one newline-terminated message. Lines longer than
// maxLineBytes are drained to the newline and reported as tooLong so the
// stream stays in sync.
func readLine(r *bufio.Reader) (line []byte, tooLong bool, err error) {
	for {
		chunk, readErr := r.ReadSlice('\n')
		if !tooLong {
			if len(line)+len(chunk) > maxLineBytes {
				tooLong = true
				line = nil
			} else {
				line = append(line, chunk...)
			}
		}
		if errors.Is(readErr, bufio.ErrBufferFull) {
			continue
		}
		return bytes.TrimRight(line, "\r\n"), tooLong, readErr
	}
}

func (s *session) handleLine(ctx context.Context, line []byte) *rpcResponse {
	var req rpcRequest
	if err := json.Unmarshal(line, &req); err != nil {
		if !json.Valid(line) {
			return errorResponse(nil, errParseError, "parse error: message is not valid JSON")
		}
		return errorResponse(nil, errInvalidRequest, "invalid request: message must be a JSON-RPC object")
	}
	isNotification := len(req.ID) == 0
	if req.JSONRPC != "2.0" || req.Method == "" {
		if isNotification {
			return nil
		}
		return errorResponse(req.ID, errInvalidRequest, "invalid request: jsonrpc must be 2.0 and method is required")
	}
	result, rpcErr := s.dispatch(ctx, req)
	if isNotification {
		return nil
	}
	if rpcErr != nil {
		return &rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: rpcErr}
	}
	return &rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
}

func (s *session) dispatch(ctx context.Context, req rpcRequest) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		var params struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &params); err != nil {
				return nil, &rpcError{Code: errInvalidParams, Message: "initialize params must be an object"}
			}
		}
		version := params.ProtocolVersion
		if version == "" {
			version = defaultProtocolVersion
		}
		return map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": hostbridge.MCPServerName, "version": Version},
		}, nil
	case "notifications/initialized":
		return nil, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		list := make([]map[string]any, 0)
		for _, t := range allTools() {
			list = append(list, map[string]any{"name": t.Name, "description": t.Description, "inputSchema": t.Schema})
		}
		return map[string]any{"tools": list}, nil
	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, &rpcError{Code: errInvalidParams, Message: "tools/call params must be an object with a name"}
		}
		if params.Arguments == nil {
			params.Arguments = map[string]any{}
		}
		text, isError := s.callTool(ctx, params.Name, params.Arguments)
		return map[string]any{
			"content": []map[string]string{{"type": "text", "text": text}},
			"isError": isError,
		}, nil
	default:
		return nil, &rpcError{Code: errMethodNotFound, Message: fmt.Sprintf("method not found: %s", req.Method)}
	}
}

func errorResponse(id json.RawMessage, code int, message string) *rpcResponse {
	return &rpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: message}}
}

// ParseServeArgs parses the arguments after `mcp serve`, which accepts exactly
// one `--host <claude|agy>` pair.
func ParseServeArgs(args []string) (hostbridge.Host, error) {
	const usage = "usage: cortex-ia mcp serve --host <claude|agy>"
	var raw string
	found := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--host" && i+1 < len(args):
			raw = args[i+1]
			i++
		case strings.HasPrefix(arg, "--host="):
			raw = strings.TrimPrefix(arg, "--host=")
		default:
			return "", fmt.Errorf("unknown argument: %s (%s)", arg, usage)
		}
		if found {
			return "", fmt.Errorf("--host may be given only once (%s)", usage)
		}
		found = true
	}
	if !found {
		return "", fmt.Errorf("--host is required (%s)", usage)
	}
	return hostbridge.ParseHost(raw)
}
