package mongo

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// What a MongoDB server is doing, and what it was started with. Nothing here touches `sqlkit.Watch`:
// there is no statement to spell, every one of these being a **command**, and a command's arguments
// are values in a document rather than text in a clause — which is why nothing in this file quotes
// anything on its way to the server.
//
// **What is in flight is an operation, not a connection.** `currentOp` answers with one document per
// operation the server is *running*; an idle connection is doing nothing and so is not one of them.
// That is the honest reading of "what is this server doing", and it is why this engine cancels an
// operation (`killOp`) and cannot end a connection at all: a MongoDB client disconnects itself.
//
// **A status is a document, and a document is not a grid.** `serverStatus` and `getParameter` both
// answer with nested documents, so both are flattened onto **dotted paths** — `wiredTiger.cache.bytes
// read into cache` — which is the same notation a reader would use to ask for one field of it. A
// nested document becomes rows; an array stays one value, spelled in the shell's own notation like
// every other value this adapter draws.

// Processes reads the operations in flight. It goes to `admin`, which is the only database the server
// will answer `currentOp` from — a reader whose credentials reach one database alone therefore meets
// the server's own refusal, which names the privilege.
func (s *session) Processes(ctx context.Context) (driver.ProcessList, error) {
	raw, err := s.adminCommand(ctx, bson.D{{Key: "currentOp", Value: 1}})
	if err != nil {
		return driver.ProcessList{}, err
	}

	list := driver.ProcessList{
		Columns: []driver.Column{
			{Name: "ID", Type: "string", FullType: "string"},
			{Name: "Operation", Type: "string", FullType: "string"},
			{Name: "Namespace", Type: "string", FullType: "string"},
			{Name: "Running", Type: "integer", FullType: "integer"},
			{Name: "Client", Type: "string", FullType: "string"},
			{Name: "Application", Type: "string", FullType: "string"},
			{Name: "Description", Type: "string", FullType: "string"},
			{Name: "Command", Type: "document", FullType: "document"},
		},
		Rows: [][]*string{},
		Refs: []driver.ProcessRef{},
	}

	operations, err := bson.Raw(raw).LookupErr("inprog")
	if err != nil {
		return list, nil
	}
	values, err := bson.Raw(operations.Array()).Values()
	if err != nil {
		return list, nil
	}

	ours := s.ownClient(ctx)
	for _, value := range values {
		operation, ok := value.DocumentOK()
		if !ok {
			continue
		}

		id := opID(operation.Lookup("opid"))
		client := textField(operation, "client")
		list.Rows = append(list.Rows, []*string{
			cellOf(id),
			cell(operation.Lookup("op")),
			cell(operation.Lookup("ns")),
			cell(operation.Lookup("secs_running")),
			cellOf(client),
			cell(operation.Lookup("appName")),
			cell(operation.Lookup("desc")),
			cell(operation.Lookup("command")),
		})
		list.Refs = append(list.Refs, driver.ProcessRef{
			ID: id,
			// A MongoDB server will not say which operation is the caller's, so what marks one as ours
			// is the *address* the connection was seen at — `whatsmyuri` against the operation's own
			// client — which is exactly as far as this engine can be asked.
			Own: ours != "" && client == ours,
		})
	}
	return list, nil
}

// ProcessStatements spells the kills as the commands a reader would type into the tab beside this
// one. An operation this engine cannot kill contributes nothing, and ending a connection contributes
// nothing at all — see KillProcesses, which refuses it by name.
func (*session) ProcessStatements(kills []driver.ProcessKill) []string {
	statements := make([]string, 0, len(kills))
	for _, kill := range kills {
		if kill.Action != domain.ProcessCancel || kill.Process.ID == "" {
			continue
		}
		statements = append(statements,
			"db.adminCommand({ killOp: 1, op: "+valueText(opValue(kill.Process.ID))+" })")
	}
	return statements
}

// KillProcesses runs exactly what ProcessStatements showed, in that order, stopping at the first
// failure. Ending a *connection* is refused naming it: this server has no command that closes
// somebody else's client, and killing the operation on it is a different thing rather than a near
// enough one.
func (s *session) KillProcesses(ctx context.Context, kills []driver.ProcessKill) error {
	for _, kill := range kills {
		if kill.Action != domain.ProcessCancel {
			return domain.Unsupported(
				"ending a connection: a MongoDB client is disconnected by the client")
		}
		if kill.Process.ID == "" {
			return domain.ValidationError{
				Field:   "process",
				Message: "is not an operation this server named",
			}
		}
	}

	for _, kill := range kills {
		command := bson.D{{Key: "killOp", Value: 1}, {Key: "op", Value: opValue(kill.Process.ID)}}
		if _, err := s.adminCommand(ctx, command); err != nil {
			return err
		}
	}
	return nil
}

// ServerStatus is `serverStatus`, flattened. It goes to whichever database the session starts in
// rather than always to `admin`, for the reason ServerInfo does: the server will answer this one from
// any database, and a reader scoped to theirs can still ask.
func (s *session) ServerStatus(ctx context.Context) (driver.ServerFacts, error) {
	raw, err := s.runCommand(ctx, driver.Namespace{}, bson.D{{Key: "serverStatus", Value: 1}})
	if err != nil {
		return driver.ServerFacts{}, err
	}
	return flatten(raw), nil
}

// ServerVariables is every parameter the server will report, flattened. `getParameter` is an admin
// command, so unlike the status above this one is asked of `admin` and nowhere else.
func (s *session) ServerVariables(ctx context.Context) (driver.ServerFacts, error) {
	raw, err := s.adminCommand(ctx, bson.D{{Key: "getParameter", Value: "*"}})
	if err != nil {
		return driver.ServerFacts{}, err
	}
	return flatten(raw), nil
}

// adminCommand runs a command that is about the **server** rather than about a database, and there is
// exactly one place those go.
func (s *session) adminCommand(ctx context.Context, command bson.D) (bson.Raw, error) {
	raw, err := s.client.Database(adminDatabase).RunCommand(ctx, command).Raw()
	if err != nil {
		return nil, translateError(err)
	}
	return raw, nil
}

// ownClient is the address this connection was seen at, which is the nearest this engine has to
// "which of these is ours". It answers with nothing rather than an error, a mark being worth less
// than the list it is on.
func (s *session) ownClient(ctx context.Context) string {
	raw, err := s.adminCommand(ctx, bson.D{{Key: "whatsmyuri", Value: 1}})
	if err != nil {
		return ""
	}

	you, err := bson.Raw(raw).LookupErr("you")
	if err != nil {
		return ""
	}
	address, _ := you.StringValueOK()
	return address
}

// flatten turns a command's answer into rows of name and value. A nested document becomes its own
// rows under a dotted path; everything else is one value, spelled the way every value in this adapter
// is. `ok` is dropped: it is the command reporting that it worked, not a fact about the server.
//
// The rows come out in **name order** rather than in the server's, because what a reader does with
// four hundred of them is look one up.
func flatten(raw bson.Raw) driver.ServerFacts {
	facts := driver.ServerFacts{
		Columns: []driver.Column{
			{Name: "Name", Type: "string", FullType: "string"},
			{Name: "Value", Type: "string", FullType: "string"},
		},
		Rows: [][]*string{},
	}

	pairs := map[string]string{}
	walk(raw, "", pairs)

	names := make([]string, 0, len(pairs))
	for name := range pairs {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		value := pairs[name]
		facts.Rows = append(facts.Rows, []*string{cellOf(name), cellOf(value)})
	}
	return facts
}

func walk(raw bson.Raw, prefix string, into map[string]string) {
	elements, err := raw.Elements()
	if err != nil {
		return
	}

	for _, element := range elements {
		name, err := element.KeyErr()
		if err != nil || (prefix == "" && name == "ok") {
			continue
		}

		path := name
		if prefix != "" {
			path = prefix + "." + name
		}

		if nested, ok := element.Value().DocumentOK(); ok {
			walk(nested, path, into)
			continue
		}
		into[path] = literal(element.Value())
	}
}

// opID is an operation's own identifier as text. It is a *string* on the wire because it is one on a
// sharded cluster — `shard0000:1234` — and a number everywhere else, and `opValue` is what turns it
// back into whichever of the two the server reported.
func opID(value bson.RawValue) string {
	if text, ok := value.StringValueOK(); ok {
		return text
	}
	if number, ok := value.AsInt64OK(); ok {
		return strconv.FormatInt(number, 10)
	}
	return ""
}

// opValue is the identifier on its way back, as the type the server named it with: `killOp` matches
// the value rather than its text, so an operation reported as a number has to go back as one.
func opValue(id string) any {
	if number, err := strconv.ParseInt(id, 10, 64); err == nil {
		return number
	}
	return id
}

// textField is one string field of a document, or the empty string for anything else. It is for the two
// facts this file compares rather than draws — the client's address — and never for a cell.
func textField(raw bson.Raw, name string) string {
	value, err := raw.LookupErr(name)
	if err != nil {
		return ""
	}
	text, _ := value.StringValueOK()
	return strings.TrimSpace(text)
}

// cellOf is text this side built on its way into a cell, where `cell` is a value the server sent.
func cellOf(text string) *string {
	return &text
}

// mongoMetrics is what the four charts are drawn from, named by the **dotted paths** `serverStatus`
// flattens to — which is the same notation the Status pane draws and the one a reader would use to
// ask for a field of it.
//
// This engine is the only one of the six that counts every panel exactly: bytes on its own sockets,
// operations by kind, and documents by what happened to them. The queries panel is `opcounters`,
// which is what was *asked for*, and the throughput panel is `metrics.document`, which is what the
// server actually moved — a `find` matching a million documents is one query and a million rows.
var mongoMetrics = []driver.MetricSpec{
	{Panel: domain.MetricSessions, Name: "connections.current", Label: "Open",
		Kind: domain.MetricGauge, Unit: domain.MetricCount},
	{Panel: domain.MetricSessions, Name: "connections.active", Label: "Active",
		Kind: domain.MetricGauge, Unit: domain.MetricCount},
	{Panel: domain.MetricSessions, Name: "globalLock.activeClients.readers", Label: "Reading",
		Kind: domain.MetricGauge, Unit: domain.MetricCount},
	{Panel: domain.MetricSessions, Name: "globalLock.activeClients.writers", Label: "Writing",
		Kind: domain.MetricGauge, Unit: domain.MetricCount},

	{Panel: domain.MetricTraffic, Name: "network.bytesIn", Label: "Received from clients",
		Kind: domain.MetricCounter, Unit: domain.MetricBytes},
	{Panel: domain.MetricTraffic, Name: "network.bytesOut", Label: "Sent to clients",
		Kind: domain.MetricCounter, Unit: domain.MetricBytes},

	{Panel: domain.MetricQueries, Name: "opcounters.query", Label: "Queries",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricQueries, Name: "opcounters.insert", Label: "Inserts",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricQueries, Name: "opcounters.update", Label: "Updates",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricQueries, Name: "opcounters.delete", Label: "Deletes",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricQueries, Name: "opcounters.getmore", Label: "Get more",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricQueries, Name: "opcounters.command", Label: "Commands",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},

	{Panel: domain.MetricThroughput, Name: "metrics.document.returned", Label: "Documents returned",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricThroughput, Name: "metrics.document.inserted", Label: "Documents inserted",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricThroughput, Name: "metrics.document.updated", Label: "Documents updated",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricThroughput, Name: "metrics.document.deleted", Label: "Documents deleted",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricThroughput, Name: "metrics.queryExecutor.scanned",
		Label: "Index keys scanned", Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricThroughput, Name: "metrics.queryExecutor.scannedObjects",
		Label: "Documents scanned", Kind: domain.MetricCounter, Unit: domain.MetricCount},
}

// ServerMetrics is the four charts' numbers out of one `serverStatus`. It goes wherever the status
// pane's read goes, for that read's reason: the server answers this one from any database, and a
// reader scoped to theirs can still ask.
func (s *session) ServerMetrics(ctx context.Context) (driver.ServerMetrics, error) {
	raw, err := s.runCommand(ctx, driver.Namespace{}, bson.D{{Key: "serverStatus", Value: 1}})
	if err != nil {
		return driver.ServerMetrics{}, err
	}

	pairs := map[string]string{}
	walk(raw, "", pairs)
	return driver.ServerMetrics{Metrics: driver.PickMetrics(mongoMetrics, pairs)}, nil
}
