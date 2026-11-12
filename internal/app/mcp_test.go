package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/history"
	historyfile "github.com/khanhsnd/vibe-db/internal/history/jsonfile"
	profilefile "github.com/khanhsnd/vibe-db/internal/profile/jsonfile"
	"github.com/khanhsnd/vibe-db/internal/session"
)

// agentApp is an app with two SQLite connections saved, only one of them open to MCP — and that one
// saved as read *and write*, which is what proves an agent is held to reading regardless.
func agentApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()

	file := filepath.Join(dir, "shop.db")
	db, err := sql.Open("sqlite", file)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT);
		INSERT INTO users (name) VALUES ('ann'), ('bob');`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	profiles := profilefile.New(filepath.Join(dir, "connections.json"))
	for _, p := range []domain.ConnectionProfile{
		{ID: "a", Name: "Shop", MCP: true},
		{ID: "b", Name: "Hidden"},
	} {
		p.Engine = domain.EngineSQLite
		p.Database = file
		p.Auth = domain.AuthPassword
		p.TLS.Mode = domain.TLSPreferred
		if err := profiles.Save(p); err != nil {
			t.Fatal(err)
		}
	}

	sessions := session.NewManager(engines())
	return &App{
		Connections: NewConnectionAPI(profiles, nil, sessions, nil, nil, nil),
		Queries: NewQueryAPI(sessions, historyfile.New(filepath.Join(dir, "history.json")), nil,
			history.NewLog()),
		sessions: sessions,
	}
}

// call runs one tool through the whole server and answers with its text and whether it failed.
func call(t *testing.T, app *App, tool string, arguments string) (string, bool) {
	t.Helper()
	line := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + tool +
		`","arguments":` + arguments + `}}`

	var out bytes.Buffer
	if err := app.ServeMCP(context.Background(), strings.NewReader(line), &out); err != nil {
		t.Fatal(err)
	}

	var reply struct {
		Result struct {
			Content []struct{ Text string } `json:"content"`
			IsError bool                    `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out.Bytes(), &reply); err != nil {
		t.Fatalf("decode %s: %v", out.String(), err)
	}
	return reply.Result.Content[0].Text, reply.Result.IsError
}

func TestAnAgentSeesOnlyTheConnectionsOpenToIt(t *testing.T) {
	text, failed := call(t, agentApp(t), "list_connections", `{}`)

	if failed || !strings.Contains(text, `"Shop"`) || strings.Contains(text, "Hidden") {
		t.Fatalf("want Shop alone, got %s", text)
	}

	_, failed = call(t, agentApp(t), "list_tables", `{"connection":"Hidden"}`)
	if !failed {
		t.Fatal("want a connection not open to MCP to be refused")
	}
}

func TestAnAgentReadsATable(t *testing.T) {
	app := agentApp(t)

	tables, failed := call(t, app, "list_tables", `{"connection":"shop"}`)
	if failed || !strings.Contains(tables, `"users"`) {
		t.Fatalf("want the users table, got %s", tables)
	}

	rows, failed := call(t, app, "query", `{"connection":"Shop","sql":"SELECT name FROM users ORDER BY id","max_rows":1}`)
	if failed || !strings.Contains(rows, `"ann"`) || strings.Contains(rows, `"bob"`) ||
		!strings.Contains(rows, `"truncated":true`) {
		t.Fatalf("want one row of two, marked truncated, got %s", rows)
	}
}

func TestAnAgentCannotWriteOnAConnectionSavedReadWrite(t *testing.T) {
	text, failed := call(t, agentApp(t), "query", `{"connection":"Shop","sql":"DELETE FROM users"}`)

	if !failed || !strings.HasPrefix(text, string(CodeReadOnly)) {
		t.Fatalf("want the delete refused as read-only, got %s", text)
	}
}
