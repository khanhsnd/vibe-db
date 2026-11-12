package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func serve(t *testing.T, server Server, lines ...string) []map[string]any {
	t.Helper()
	var out bytes.Buffer
	if err := server.Serve(context.Background(), strings.NewReader(strings.Join(lines, "\n")), &out); err != nil {
		t.Fatalf("serve: %v", err)
	}

	var replies []map[string]any
	decoder := json.NewDecoder(&out)
	for decoder.More() {
		var reply map[string]any
		if err := decoder.Decode(&reply); err != nil {
			t.Fatalf("decode: %v", err)
		}
		replies = append(replies, reply)
	}
	return replies
}

func echo() Server {
	return Server{Name: "test", Version: "1", Tools: []Tool{{
		Name:  "echo",
		Input: map[string]any{"type": "object"},
		Call: func(_ context.Context, raw json.RawMessage) (string, error) {
			if strings.Contains(string(raw), "fail") {
				return "", errors.New("refused")
			}
			return string(raw), nil
		},
	}}}
}

func TestInitializeAnswersWithTheVersionTheClientAskedFor(t *testing.T) {
	replies := serve(t, echo(),
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`,
		`{"jsonrpc":"2.0","id":2,"method":"initialize","params":{"protocolVersion":"1999-01-01"}}`,
	)

	if got := replies[0]["result"].(map[string]any)["protocolVersion"]; got != "2025-06-18" {
		t.Fatalf("want the client's version, got %v", got)
	}
	if got := replies[1]["result"].(map[string]any)["protocolVersion"]; got != latestVersion {
		t.Fatalf("want the latest version for one it does not know, got %v", got)
	}
}

func TestANotificationIsNotAnswered(t *testing.T) {
	replies := serve(t, echo(),
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":7,"method":"ping"}`,
	)

	if len(replies) != 1 || replies[0]["id"] != float64(7) {
		t.Fatalf("want only the ping answered, got %v", replies)
	}
}

func TestAToolFailureIsAnAnswerAndNotAProtocolError(t *testing.T) {
	replies := serve(t, echo(),
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo","arguments":{"say":"hi"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"echo","arguments":{"say":"fail"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"nope"}}`,
	)

	ok := replies[0]["result"].(map[string]any)
	if ok["isError"] != false || !strings.Contains(ok["content"].([]any)[0].(map[string]any)["text"].(string), "hi") {
		t.Fatalf("want the tool's text, got %v", ok)
	}

	failed := replies[1]["result"].(map[string]any)
	if failed["isError"] != true || failed["content"].([]any)[0].(map[string]any)["text"] != "refused" {
		t.Fatalf("want the failure as a result the model reads, got %v", failed)
	}

	if replies[2]["error"] == nil {
		t.Fatalf("want an unknown tool to be a protocol error, got %v", replies[2])
	}
}

func TestToolsAreListedWithTheirSchema(t *testing.T) {
	replies := serve(t, echo(), `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)

	tools := replies[0]["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 1 || tools[0].(map[string]any)["inputSchema"] == nil {
		t.Fatalf("want the one tool with its schema, got %v", tools)
	}
}
