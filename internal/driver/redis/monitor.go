package redis

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// What a Redis server is doing, and what it was started with. Nothing here touches `sqlkit.Watch`:
// there is no statement to spell, every one of these being a command whose arguments are its own
// words — the arrangement `account.go` has, and for the same reason.
//
// **A client is what Redis calls a connection**, and `CLIENT LIST` is one line per client, in the
// same `name=value` shape `INFO` uses. That line is the whole of what the server keeps about one, so
// it is read apart into columns here rather than shown raw.
//
// **There is nothing to cancel.** A Redis server runs one command at a time and to completion, so
// there is no statement in flight for a second connection to stop: what a reader can end is the
// client, `CLIENT KILL ID`. The two exceptions are a script and a function, which have `SCRIPT KILL`
// and `FUNCTION KILL` of their own and are typed into a tab rather than pressed here — a running
// script is not a row of this list.

// clientColumns is what a `CLIENT LIST` line says, in the order a reader reads it. They are the same
// columns on every Redis server there is, which is what a key-value store has instead of a catalogue —
// exactly as `keyColumns` is for its one table.
var clientColumns = []struct {
	Column string
	Field  string
}{
	{"ID", "id"},
	{"Address", "addr"},
	{"Name", "name"},
	{"User", "user"},
	{"Database", "db"},
	{"Age", "age"},
	{"Idle", "idle"},
	{"Command", "cmd"},
	{"Flags", "flags"},
	// How many commands are queued in a MULTI on this client, `-1` being none — which is the nearest
	// thing a Redis client has to "in a transaction".
	{"Queued", "multi"},
}

func (s *session) Processes(ctx context.Context) (driver.ProcessList, error) {
	client, err := s.client(driver.Namespace{})
	if err != nil {
		return driver.ProcessList{}, err
	}

	listing, err := client.ClientList(ctx).Result()
	if err != nil {
		return driver.ProcessList{}, translateError(err)
	}

	list := driver.ProcessList{
		Columns: make([]driver.Column, 0, len(clientColumns)),
		Rows:    [][]*string{},
		Refs:    []driver.ProcessRef{},
	}
	for _, column := range clientColumns {
		list.Columns = append(list.Columns, driver.Column{
			Name: column.Column, Type: "string", FullType: "string",
		})
	}

	ours := s.ownProcess(ctx)
	for _, line := range strings.Split(listing, "\n") {
		fields := clientFields(line)
		if len(fields) == 0 {
			continue
		}

		row := make([]*string, 0, len(clientColumns))
		for _, column := range clientColumns {
			row = append(row, cell(fields[column.Field]))
		}
		list.Rows = append(list.Rows, row)

		id := fields["id"]
		list.Refs = append(list.Refs, driver.ProcessRef{ID: id, Own: ours != "" && id == ours})
	}
	return list, nil
}

// ProcessStatements spells the kills as the commands a reader would type into the tab beside this
// one. Cancelling contributes nothing, this engine having nothing to cancel — see KillProcesses,
// which refuses it by name.
func (*session) ProcessStatements(kills []driver.ProcessKill) []string {
	statements := make([]string, 0, len(kills))
	for _, kill := range kills {
		if kill.Action == domain.ProcessCancel || !isNumber(kill.Process.ID) {
			continue
		}
		statements = append(statements, "CLIENT KILL ID "+kill.Process.ID)
	}
	return statements
}

func (s *session) KillProcesses(ctx context.Context, kills []driver.ProcessKill) error {
	for _, kill := range kills {
		if kill.Action == domain.ProcessCancel {
			return domain.Unsupported(
				"cancelling a command: a Redis server runs one at a time and to completion")
		}
		if !isNumber(kill.Process.ID) {
			return domain.ValidationError{Field: "process", Message: "is not a client this server names"}
		}
	}

	client, err := s.client(driver.Namespace{})
	if err != nil {
		return err
	}

	for _, kill := range kills {
		// The id goes as an **argument** rather than into text, so nothing here is quoted: what makes
		// this safe is the protocol, and the check above is what keeps the preview and the run alike.
		if err := client.Do(ctx, "CLIENT", "KILL", "ID", kill.Process.ID).Err(); err != nil {
			return translateError(err)
		}
	}
	return nil
}

// ownProcess is the client this read came back on, which is what marks a row as one of ours. A
// session holds a pool per database, so it names one of ours rather than the only one — see
// `driver.Monitor.Processes`.
func (s *session) ownProcess(ctx context.Context) string {
	client, err := s.client(driver.Namespace{})
	if err != nil {
		return ""
	}

	id, err := client.ClientID(ctx).Result()
	if err != nil {
		return ""
	}
	return strconv.FormatInt(id, 10)
}

// ServerStatus is `INFO`, which is where Redis keeps every fact about itself that is not a key. The
// section is a column of its own rather than part of the name, because that is how the server groups
// them and how a reader reads them: `used_memory` is one of a dozen numbers about memory.
//
// `INFO all` is asked for first — it carries the per-command statistics, which is the half of this a
// reader watching a busy server actually wants — and a server that will not answer it falls back to
// the default sections rather than to nothing.
func (s *session) ServerStatus(ctx context.Context) (driver.ServerFacts, error) {
	client, err := s.client(driver.Namespace{})
	if err != nil {
		return driver.ServerFacts{}, err
	}

	info, err := client.Info(ctx, "all").Result()
	if err != nil {
		if info, err = client.Info(ctx).Result(); err != nil {
			return driver.ServerFacts{}, translateError(err)
		}
	}

	facts := driver.ServerFacts{
		Columns: []driver.Column{
			{Name: "Section", Type: "string", FullType: "string"},
			{Name: "Name", Type: "string", FullType: "string"},
			{Name: "Value", Type: "string", FullType: "string"},
		},
		Rows: [][]*string{},
	}

	section := ""
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "":
			continue
		case strings.HasPrefix(line, "#"):
			section = strings.TrimSpace(strings.TrimPrefix(line, "#"))
			continue
		}

		name, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		facts.Rows = append(facts.Rows, []*string{cell(section), cell(name), cell(value)})
	}
	return facts, nil
}

// ServerVariables is `CONFIG GET *`, which is the whole of what a Redis server was started with. A
// hosted Redis usually refuses `CONFIG` outright, and that refusal is the server's own words rather
// than an empty list: a reader who cannot read the configuration is told so instead of being shown a
// server that appears to have none.
func (s *session) ServerVariables(ctx context.Context) (driver.ServerFacts, error) {
	client, err := s.client(driver.Namespace{})
	if err != nil {
		return driver.ServerFacts{}, err
	}

	config, err := client.ConfigGet(ctx, "*").Result()
	if err != nil {
		return driver.ServerFacts{}, translateError(err)
	}

	names := make([]string, 0, len(config))
	for name := range config {
		names = append(names, name)
	}
	sort.Strings(names)

	facts := driver.ServerFacts{
		Columns: []driver.Column{
			{Name: "Name", Type: "string", FullType: "string"},
			{Name: "Value", Type: "string", FullType: "string"},
		},
		Rows: make([][]*string, 0, len(names)),
	}
	for _, name := range names {
		facts.Rows = append(facts.Rows, []*string{cell(name), cell(config[name])})
	}
	return facts, nil
}

// clientFields reads one `CLIENT LIST` line into the pairs it holds. A line is space-separated
// `name=value`, and a value may hold an `=` of its own — a `cmd` of `client|list`, an address — so
// only the first is the separator.
func clientFields(line string) map[string]string {
	fields := map[string]string{}
	for _, word := range strings.Fields(line) {
		name, value, found := strings.Cut(word, "=")
		if found {
			fields[name] = value
		}
	}
	return fields
}

// isNumber says the id is one, which is what stands between a value off the wire and a command's
// arguments. It is `sqlkit`'s check under another name, and it is here rather than borrowed because
// nothing else in this adapter needs anything from that package's SQL half.
func isNumber(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

// redisMetrics is what the four charts are drawn from, named by the fields `INFO` reports them under.
// Every one of them is in the default sections, which is why this read asks for those rather than for
// `INFO all`: the per-command statistics that make the Status pane worth reading are several hundred
// lines a chart never looks at, and this read happens every two seconds.
//
// **A Redis server has no idle connection to distinguish**, so the sessions panel is what it does
// keep: how many clients are attached, and how many of those are blocked waiting on a list or a
// stream. The throughput panel is the key space, which is this engine's answer to "what did that work
// move" — a hit and a miss are the two halves of whether the data asked for was there.
var redisMetrics = []driver.MetricSpec{
	{Panel: domain.MetricSessions, Name: "connected_clients", Label: "Clients",
		Kind: domain.MetricGauge, Unit: domain.MetricCount},
	{Panel: domain.MetricSessions, Name: "blocked_clients", Label: "Blocked",
		Kind: domain.MetricGauge, Unit: domain.MetricCount},
	{Panel: domain.MetricSessions, Name: "watching_clients", Label: "Watching",
		Kind: domain.MetricGauge, Unit: domain.MetricCount},

	{Panel: domain.MetricTraffic, Name: "total_net_input_bytes", Label: "Received from clients",
		Kind: domain.MetricCounter, Unit: domain.MetricBytes},
	{Panel: domain.MetricTraffic, Name: "total_net_output_bytes", Label: "Sent to clients",
		Kind: domain.MetricCounter, Unit: domain.MetricBytes},

	{Panel: domain.MetricQueries, Name: "total_commands_processed", Label: "Commands",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricQueries, Name: "total_error_replies", Label: "Errors",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricQueries, Name: "total_connections_received", Label: "Connections",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricQueries, Name: "rejected_connections", Label: "Rejected",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},

	{Panel: domain.MetricThroughput, Name: "keyspace_hits", Label: "Key space hits",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricThroughput, Name: "keyspace_misses", Label: "Key space misses",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricThroughput, Name: "expired_keys", Label: "Keys expired",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
	{Panel: domain.MetricThroughput, Name: "evicted_keys", Label: "Keys evicted",
		Kind: domain.MetricCounter, Unit: domain.MetricCount},
}

// ServerMetrics is the four charts' numbers out of the default `INFO` sections.
func (s *session) ServerMetrics(ctx context.Context) (driver.ServerMetrics, error) {
	client, err := s.client(driver.Namespace{})
	if err != nil {
		return driver.ServerMetrics{}, err
	}

	info, err := client.Info(ctx).Result()
	if err != nil {
		return driver.ServerMetrics{}, translateError(err)
	}
	return driver.ServerMetrics{Metrics: driver.PickMetrics(redisMetrics, infoPairs(info))}, nil
}

// infoPairs reads an `INFO` answer into the pairs it holds, dropping the section headings it groups
// them under. A key-space line — `db0:keys=1,expires=0` — is a pair whose value is not a number, and
// is left where it is rather than special-cased: nothing is looking for it, and `driver.PickMetrics`
// is what decides that a value it cannot read is a metric this server has not got.
func infoPairs(info string) map[string]string {
	pairs := map[string]string{}
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if name, value, found := strings.Cut(line, ":"); found {
			pairs[name] = value
		}
	}
	return pairs
}
