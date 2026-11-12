# An AI agent reading a connection (MCP)

[← Features and UI behaviour](../features.md)

## What it is

**`vibe-db mcp` is this app with no window, answering an AI agent over the Model Context Protocol.**
A reader chatting with Claude says *"use vibe-db to read the `orders` table on Shop"*, and the agent
finds the connection by its name, reads the table's shape and runs a query — through the
connections, the credential store and the guards the window already uses.

It is registered once, in a terminal. The connection dialog's **MCP** tab shows the line for the copy
of the app that is running, so the path is right wherever it was installed:

```bash
claude mcp add vibe-db -- "/Applications/vibe-db.app/Contents/MacOS/vibe-db" mcp
```

Any client that starts an MCP server as a program over stdio takes the same program and the same
argument. **The app does not have to be open**: the client starts its own copy, and that copy ends
when the client hangs up.

## Which connections an agent sees

**None, until the reader says so.** Each connection has a checkbox on its MCP tab — *Let AI agents read
this connection* — saved as `mcp` in `connections.json`, and off for every connection, including every
one saved before this existed. A connection that is not ticked is not listed and cannot be named: a
connection an agent can see is one a prompt can name, and a prompt is not always the reader's own.

What an agent is told about one is its **name, engine, environment and target** (`database@host:port`)
— never a password, which it never needs: the session is dialled with whatever the credential store
holds, exactly as a [scheduled job](automation.md) dials one. A profile that asks for its password every
time is dialled with none and fails the way the server says it does.

## Always read-only

**An agent is held to reading, whatever `Access` says.** The rule is
`domain.ConnectionProfile.ForAgent`: the profile an agent is given is the stored one with `Access` set
to read-only, so the session is wrapped by the same guard a [read-only connection](read-only.md#what-is-refused)
is, and refuses the same things — a write, a schema change, `WITH`, a kill — before anything is sent.

The reason is the confirmation, not the agent. A statement the window would ask about on production
asks a person, and there is no person in front of an MCP call to ask; a refusal is the only answer
left that does not default to allowing. Turning `Access` back to read-and-write changes nothing for
an agent. **A read-only account on the server is still the protection**, for the reason the read-only
page gives.

## The tools

Every tool is a bound method the window calls — `ListDatabases`, `LoadTableSchemas`,
`LoadTableStructure`, `QueryAPI.Run` — so an agent reads what the tree and a query tab would have
shown. A statement it runs is kept in the [history](results.md#the-history-pane) like any other — on
disk, so the window shows it the next time it reads the history; the [log](log.md) is a window's own
and does not see it.

| Tool | Answers with |
|---|---|
| `list_connections` | The connections open to MCP — name, engine, environment, target |
| `list_databases` | The databases on the connection's server |
| `list_schemas` | The schemas of one database; empty where a database is its schema |
| `list_tables` | Each table of a namespace with its column names |
| `describe_table` | One table's columns with types, its indexes and its foreign keys |
| `query` | The rows of one statement, or what each statement of a script came to |

Every tool but the first takes `connection` — the name, matched without regard to case — and
optionally `database` and `schema`; a database left out is the connection's own. `query` takes the
statement in the engine's own language (SQL, a MongoDB shell command, a Redis command) and
**`max_rows`: 100 when left out, never more than 1,000**. Every row is read by a model and paid for in
its context, so the default is a look at the data and `truncated` says when there was more.

A failure is an **answer the agent reads**, not a broken connection: the [error map](../errors.md)'s
code and sentence — `READ_ONLY: …`, `BAD_CREDENTIALS: …` — so it can say what went wrong or try
something else.

## How it is built

`internal/mcp` speaks the protocol and nothing else: one JSON-RPC message to a line over stdin and
stdout, and `initialize`, `ping`, `tools/list` and `tools/call`. No SDK is taken, for the reason
`internal/sigv4` takes none — it is a handful of JSON shapes. It knows nothing about a database;
`internal/app/mcp.go` hands it the tools, and `main.go` runs it in place of the window when the first
argument is `mcp`. A session is opened the first time a connection is named and kept until the client
hangs up, so a question asked in five calls dials the server once.

Not there yet: an agent cannot write, even where the reader would allow it; nothing marks a statement
in the history as the agent's rather than the reader's; and on Windows the program is built as a
windowed application, which an MCP client starting it over pipes has not been tried against.
