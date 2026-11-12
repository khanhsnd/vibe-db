package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/mcp"
)

// agentRows is how many rows a query hands an agent when it does not say, and agentRowCap is the
// most it may ask for. Every row is read by a model and paid for in its context, so the default is a
// look at the data rather than the data.
const (
	agentRows   = 100
	agentRowCap = 1000
)

const agentInstructions = "Vibe DB's saved database connections. Call list_connections first: a " +
	"connection is named by its name. Every connection here is read-only — a statement that writes " +
	"is refused before it reaches the server. Read a table's columns with list_tables or " +
	"describe_table before writing a query, and keep queries bounded."

// ServeMCP answers an AI agent over stdio until it hangs up — see docs/features/mcp.md.
//
// **Every tool is a bound method the window calls**, so what an agent reads is exactly what the
// tree and a query tab would have shown, through the same seams: the read-only guard, the history,
// the error map. Only a connection the reader opened to MCP is visible, and it is opened as
// domain.ConnectionProfile.ForAgent answers — read-only, whatever the reader chose for themselves.
func (a *App) ServeMCP(ctx context.Context, in io.Reader, out io.Writer) error {
	a.Connections.useContext(ctx)
	a.Queries.useContext(ctx)
	defer a.sessions.CloseAll()

	agent := &agent{connections: a.Connections, queries: a.Queries, sessions: map[string]Session{}}
	server := mcp.Server{
		Name:         "vibe-db",
		Version:      a.version,
		Instructions: agentInstructions,
		Tools:        agent.tools(),
	}
	return server.Serve(ctx, in, out)
}

// agent is one MCP client's view of the app. Its sessions live as long as the client does, so a
// question in five calls dials the server once.
type agent struct {
	connections *ConnectionAPI
	queries     *QueryAPI
	// sessions is keyed by profile id.
	sessions map[string]Session
}

// agentTarget is where a tool is pointed: a connection by its name, and optionally a namespace in
// it. A namespace left out is the connection's own database.
type agentTarget struct {
	Connection string `json:"connection"`
	Database   string `json:"database"`
	Schema     string `json:"schema"`
}

type agentConnection struct {
	Name        string `json:"name"`
	Engine      string `json:"engine"`
	Environment string `json:"environment,omitempty"`
	Target      string `json:"target"`
}

type agentResult struct {
	Columns      []string           `json:"columns,omitempty"`
	Rows         [][]*string        `json:"rows,omitempty"`
	RowsAffected int64              `json:"rowsAffected,omitempty"`
	Truncated    bool               `json:"truncated,omitempty"`
	Statements   []StatementOutcome `json:"statements,omitempty"`
}

func (g *agent) tools() []mcp.Tool {
	return []mcp.Tool{
		agentTool("list_connections",
			"Lists the database connections saved in Vibe DB that are open to MCP.",
			schema(nil),
			func(json.RawMessage) (any, error) { return g.listConnections() }),
		agentTool("list_databases",
			"Lists the databases on a connection's server.",
			schema(nil, "connection"),
			func(raw json.RawMessage) (any, error) {
				id, _, err := g.open(raw)
				if err != nil {
					return nil, err
				}
				return g.connections.ListDatabases(id)
			}),
		agentTool("list_schemas",
			"Lists the schemas of one database. Empty on an engine whose database is its schema, such as MySQL.",
			schema(nil, "connection", "database"),
			func(raw json.RawMessage) (any, error) {
				id, ns, err := g.open(raw)
				if err != nil {
					return nil, err
				}
				return g.connections.ListSchemas(id, ns.Database)
			}),
		agentTool("list_tables",
			"Lists the tables of a database (or schema), each with its column names.",
			schema(nil, "connection", "database", "schema"),
			func(raw json.RawMessage) (any, error) {
				id, ns, err := g.open(raw)
				if err != nil {
					return nil, err
				}
				return g.connections.LoadTableSchemas(id, ns)
			}),
		agentTool("describe_table",
			"Describes one table: its columns with their types, its indexes and its foreign keys.",
			schema(map[string]any{"table": stringProperty("The table's name.")},
				"connection", "database", "schema", "table"),
			func(raw json.RawMessage) (any, error) {
				var args struct {
					Table string `json:"table"`
				}
				id, ns, err := g.open(raw, &args)
				if err != nil {
					return nil, err
				}
				return g.connections.LoadTableStructure(id, ns, args.Table)
			}),
		agentTool("query",
			"Runs a read-only statement in the engine's own language — SQL, a MongoDB shell command "+
				"or a Redis command — and answers with its rows. Anything that writes is refused.",
			schema(map[string]any{
				"sql": stringProperty("The statement to run."),
				"max_rows": map[string]any{
					"type":        "integer",
					"description": fmt.Sprintf("Most rows to answer with: %d if left out, at most %d.", agentRows, agentRowCap),
				},
			}, "connection", "database", "schema", "sql", "max_rows"),
			func(raw json.RawMessage) (any, error) {
				var args struct {
					SQL     string `json:"sql"`
					MaxRows int    `json:"max_rows"`
				}
				id, ns, err := g.open(raw, &args)
				if err != nil {
					return nil, err
				}
				return g.query(id, ns, args.SQL, args.MaxRows)
			}),
	}
}

func (g *agent) listConnections() ([]agentConnection, error) {
	profiles, err := g.connections.profiles.List()
	if err != nil {
		return nil, wrapError(err)
	}

	out := []agentConnection{}
	for _, p := range profiles {
		if p.MCP {
			out = append(out, agentConnection{
				Name:        p.Name,
				Engine:      string(p.Engine),
				Environment: string(p.Environment),
				Target:      p.Target(),
			})
		}
	}
	if len(out) == 0 {
		return nil, errors.New("no connection is open to MCP yet: in Vibe DB, edit a connection " +
			"and turn on \"Let AI agents read this connection\" on its MCP tab")
	}
	return out, nil
}

// open reads a tool's target out of its arguments — and anything else the tool takes into extra —
// and answers with a session on that connection, dialling it the first time it is named.
func (g *agent) open(raw json.RawMessage, extra ...any) (string, Namespace, error) {
	var target agentTarget
	for _, into := range append([]any{&target}, extra...) {
		if err := json.Unmarshal(raw, into); err != nil {
			return "", Namespace{}, fmt.Errorf("the arguments are not what this tool takes: %w", err)
		}
	}

	profiles, err := g.connections.profiles.List()
	if err != nil {
		return "", Namespace{}, wrapError(err)
	}
	for _, p := range profiles {
		if !p.MCP || !strings.EqualFold(p.Name, strings.TrimSpace(target.Connection)) {
			continue
		}

		ns := Namespace{Database: target.Database, Schema: target.Schema}
		if ns.Database == "" {
			ns.Database = p.Database
		}
		if open, ok := g.sessions[p.ID]; ok {
			return open.ID, ns, nil
		}
		opened, err := g.connections.open(p.ForAgent(), "")
		if err != nil {
			return "", Namespace{}, err
		}
		g.sessions[p.ID] = opened
		return opened.ID, ns, nil
	}
	return "", Namespace{}, fmt.Errorf("no connection named %q is open to MCP — "+
		"list_connections names the ones that are", target.Connection)
}

func (g *agent) query(sessionID string, ns Namespace, sql string, maxRows int) (agentResult, error) {
	if maxRows <= 0 {
		maxRows = agentRows
	}
	maxRows = min(maxRows, agentRowCap)

	script, err := g.queries.Run(sessionID, QueryInput{Namespace: ns, SQL: sql, MaxRows: maxRows})
	if err != nil {
		return agentResult{}, err
	}

	result := agentResult{
		Rows:         script.Rows,
		RowsAffected: script.RowsAffected,
		Truncated:    script.Truncated,
	}
	for _, column := range script.Columns {
		result.Columns = append(result.Columns, column.Name)
	}
	// One statement is its own result; a script says what each of its statements came to, which is
	// where a statement that failed part way through says so.
	if len(script.Statements) > 1 {
		result.Statements = script.Statements
	}
	return result, nil
}

// agentTool is a tool whose answer is JSON and whose failure is the error map's own words.
func agentTool(name, description string, input map[string]any, call func(json.RawMessage) (any, error)) mcp.Tool {
	return mcp.Tool{
		Name:        name,
		Description: description,
		Input:       input,
		Call: func(_ context.Context, raw json.RawMessage) (string, error) {
			answer, err := call(raw)
			if err != nil {
				return "", agentError(err)
			}
			encoded, err := json.Marshal(answer)
			if err != nil {
				return "", err
			}
			return string(encoded), nil
		},
	}
}

// agentError is a failure as a model reads it: the code it can switch on and the sentence the window
// would have shown, never the JSON a bound method hands the frontend.
func agentError(err error) error {
	var wire wireError
	if !errors.As(err, &wire) {
		return err
	}
	message := string(wire.Code) + ": " + wire.Message
	if wire.Detail != "" {
		message += " — " + wire.Detail
	}
	return errors.New(message)
}

var agentProperties = map[string]any{
	"connection": stringProperty("The connection's name, as list_connections gives it."),
	"database":   stringProperty("The database to look in. Left out, the connection's own database."),
	"schema":     stringProperty("The schema, on an engine that has them. Left out, the database's default."),
}

// schema is a tool's input schema: the properties named, taken from extra or from the ones every
// tool shares, with the connection — and sql and table, where a tool takes them — required.
func schema(extra map[string]any, names ...string) map[string]any {
	properties := map[string]any{}
	required := []string{}
	for _, name := range names {
		if property, ok := extra[name]; ok {
			properties[name] = property
		} else {
			properties[name] = agentProperties[name]
		}
		if name == "connection" || name == "sql" || name == "table" {
			required = append(required, name)
		}
	}
	return map[string]any{"type": "object", "properties": properties, "required": required}
}

func stringProperty(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}
