// Package mcp speaks the Model Context Protocol over stdio, and nothing else: one JSON-RPC message to
// a line in, one to a line out, and the four methods a client needs to find and call a tool.
//
// It knows nothing about a database. What a tool *does* is the caller's — internal/app hands this
// package a list of tools whose bodies are the same bound methods the window calls — which is
// internal/sigv4's arrangement applied to a protocol: no SDK is taken for a handful of JSON shapes.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
)

// latestVersion is what this server answers with when a client asks for a protocol version it does
// not know. Every version listed speaks tools the same way, which is all this server offers.
const latestVersion = "2025-11-25"

var knownVersions = []string{latestVersion, "2025-06-18", "2025-03-26", "2024-11-05"}

// Tool is one thing an agent may call. Call answers with text for the agent to read; an error is
// reported to the agent as a failed call rather than as a protocol error, because a refused
// statement is an answer it should read and act on, not a broken connection.
type Tool struct {
	Name        string
	Description string
	// Input is the JSON Schema of the arguments, as the client shows it to the model.
	Input map[string]any
	Call  func(ctx context.Context, arguments json.RawMessage) (string, error)
}

type Server struct {
	Name         string
	Version      string
	Instructions string
	Tools        []Tool
}

type request struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

const (
	parseError     = -32700
	methodNotFound = -32601
	invalidParams  = -32602
)

// Serve reads requests until the input ends, which is how a client says it is done. Requests are
// answered one at a time and in order: a stdio client sends one call and waits for it.
func (s Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	reader := bufio.NewReader(in)
	encoder := json.NewEncoder(out)

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if reply, ok := s.handle(ctx, line); ok {
				if writeErr := encoder.Encode(reply); writeErr != nil {
					return writeErr
				}
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// handle answers one line. A notification — a message with no id — is answered with nothing.
func (s Server) handle(ctx context.Context, line []byte) (response, bool) {
	var req request
	if err := json.Unmarshal(line, &req); err != nil {
		return failure(json.RawMessage("null"), parseError, "not a JSON-RPC message"), true
	}
	if len(req.ID) == 0 {
		return response{}, false
	}

	switch req.Method {
	case "initialize":
		return success(req.ID, s.initialize(req.Params)), true
	case "ping":
		return success(req.ID, struct{}{}), true
	case "tools/list":
		return success(req.ID, s.list()), true
	case "tools/call":
		return s.call(ctx, req)
	default:
		return failure(req.ID, methodNotFound, "unknown method "+req.Method), true
	}
}

func (s Server) initialize(params json.RawMessage) map[string]any {
	var asked struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	_ = json.Unmarshal(params, &asked)

	version := latestVersion
	if slices.Contains(knownVersions, asked.ProtocolVersion) {
		version = asked.ProtocolVersion
	}

	return map[string]any{
		"protocolVersion": version,
		"capabilities":    map[string]any{"tools": map[string]any{}},
		"serverInfo":      map[string]any{"name": s.Name, "version": s.Version},
		"instructions":    s.Instructions,
	}
}

func (s Server) list() map[string]any {
	tools := make([]map[string]any, 0, len(s.Tools))
	for _, tool := range s.Tools {
		tools = append(tools, map[string]any{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": tool.Input,
		})
	}
	return map[string]any{"tools": tools}
}

func (s Server) call(ctx context.Context, req request) (response, bool) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return failure(req.ID, invalidParams, "tools/call needs a name and arguments"), true
	}

	index := slices.IndexFunc(s.Tools, func(tool Tool) bool { return tool.Name == params.Name })
	if index < 0 {
		return failure(req.ID, invalidParams, fmt.Sprintf("unknown tool %q", params.Name)), true
	}
	if len(params.Arguments) == 0 {
		params.Arguments = json.RawMessage("{}")
	}

	text, err := s.Tools[index].Call(ctx, params.Arguments)
	if err != nil {
		return success(req.ID, toolResult(err.Error(), true)), true
	}
	return success(req.ID, toolResult(text, false)), true
}

func toolResult(text string, failed bool) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
		"isError": failed,
	}
}

func success(id json.RawMessage, result any) response {
	return response{JSONRPC: "2.0", ID: id, Result: result}
}

func failure(id json.RawMessage, code int, message string) response {
	return response{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: message}}
}
