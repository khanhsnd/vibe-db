package redis

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// A live run against a real server, because the three things about this adapter that cannot be
// checked any other way are the three that make it a key-value one: a page is cut from a *scan* and
// only a server scans; a value is read by a different command per type and only a server has the
// types; and a command reaches the server as argv, so only a server can say the arguments arrived as
// they were meant to.
//
// It is skipped unless `VIBE_REDIS_DSN` names a server — see docs/development.md — and it works in a
// database of its own, which it empties first, so nothing it does can be mistaken for something the
// reader put there.
//
//	VIBE_REDIS_DSN=127.0.0.1:6379 go test ./internal/driver/redis -run TestLive -v
const liveEnv = "VIBE_REDIS_DSN"

// liveDatabase is the last database of a default install, which is the one least likely to hold
// anything. It is emptied at the start of the run.
const liveDatabase = "15"

func openLive(t *testing.T) (driver.Session, driver.Namespace) {
	t.Helper()

	dsn := os.Getenv(liveEnv)
	if dsn == "" {
		t.Skipf("set %s to run this against a real server", liveEnv)
	}

	profile := domain.ConnectionProfile{
		Name:     "live",
		Engine:   domain.EngineRedis,
		Database: liveDatabase,
		Auth:     domain.AuthPassword,
		TLS:      domain.TLSOptions{Mode: domain.TLSDisabled},
	}
	credentials := driver.Credentials{}

	address := dsn
	if at := strings.LastIndex(dsn, "@"); at >= 0 {
		profile.User, credentials.Password, _ = strings.Cut(dsn[:at], ":")
		address = dsn[at+1:]
	}

	host, port, found := strings.Cut(address, ":")
	if !found {
		t.Fatalf("%s = %q, want [user:password@]host:port", liveEnv, dsn)
	}
	profile.Host = host
	profile.Port, _ = strconv.Atoi(port)

	if err := profile.Validate(); err != nil {
		t.Fatalf("the live profile is not valid: %v", err)
	}

	live, err := New().Open(context.Background(), profile, credentials, nil)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = live.Close() })

	ns := driver.Namespace{Database: liveDatabase}
	if err := live.EmptyTable(context.Background(), ns, keySpace); err != nil {
		t.Fatalf("emptying the test database: %v", err)
	}
	return live, ns
}

// run is one command through the same path the query tab uses, which is how this test writes its
// own fixtures: nothing here reaches the server except the way the app would send it.
func run(t *testing.T, live driver.Session, ns driver.Namespace, command string) driver.QueryResult {
	t.Helper()

	result, err := live.Query(context.Background(), driver.Query{Namespace: ns, SQL: command})
	if err != nil {
		t.Fatalf("Query(%q) error = %v", command, err)
	}
	return result
}

func seed(t *testing.T, live driver.Session, ns driver.Namespace) {
	t.Helper()

	for _, command := range []string{
		`SET user:1 "Bob Smith"`,
		`SET user:2 "Ann Jones"`,
		`SET tmp:1 "goes away" EX 600`,
		`HSET session:4012 domain 100.96.2.29 port 4012`,
		`RPUSH queue:email first second third`,
		`SADD tags:1 red green blue`,
		`ZADD board 10 bob 20 ann`,
		`XADD events "*" kind login`,
	} {
		run(t, live, ns, command)
	}
}

func TestLiveAgainstARealServer(t *testing.T) {
	live, ns := openLive(t)
	ctx := context.Background()
	seed(t, live, ns)

	t.Run("ServerInfo", func(t *testing.T) {
		info, err := live.ServerInfo(ctx)
		if err != nil {
			t.Fatalf("ServerInfo() error = %v", err)
		}
		if info.Version == "" {
			t.Error("Version is empty")
		}
		t.Logf("server version %s", info.Version)
	})

	t.Run("Databases are numbered", func(t *testing.T) {
		names, err := live.Databases(ctx)
		if err != nil {
			t.Fatalf("Databases() error = %v", err)
		}
		if len(names) < 2 || names[0] != "0" {
			t.Fatalf("Databases() = %q, want them counting from 0", names)
		}
	})

	t.Run("Objects hold one key space", func(t *testing.T) {
		objects, err := live.Objects(ctx, ns)
		if err != nil {
			t.Fatalf("Objects() error = %v", err)
		}
		if len(objects.Tables) != 1 || objects.Tables[0] != keySpace {
			t.Errorf("Tables = %q, want just %q", objects.Tables, keySpace)
		}
	})

	t.Run("TableList counts the keys", func(t *testing.T) {
		list, err := live.TableList(ctx, ns)
		if err != nil {
			t.Fatalf("TableList() error = %v", err)
		}
		if len(list.Rows) != 1 {
			t.Fatalf("TableList() = %d rows, want 1", len(list.Rows))
		}
		if got := *list.Rows[0][1]; got != "8" {
			t.Errorf("keys = %s, want 8", got)
		}
		if got := *list.Rows[0][2]; got != "1" {
			t.Errorf("expiring = %s, want 1", got)
		}
	})

	t.Run("TableRows reads every type", func(t *testing.T) {
		rows, err := live.TableRows(ctx, driver.TablePage{Namespace: ns, Table: keySpace})
		if err != nil {
			t.Fatalf("TableRows() error = %v", err)
		}
		if rows.Total != 8 {
			t.Fatalf("Total = %d, want 8", rows.Total)
		}

		values := map[string]string{}
		kinds := map[string]string{}
		for _, row := range rows.Rows {
			values[*row[0]] = *row[2]
			kinds[*row[0]] = *row[1]
		}

		wantValue := map[string]string{
			"user:1":       "Bob Smith",
			"queue:email":  `["first","second","third"]`,
			"session:4012": `{"domain":"100.96.2.29","port":"4012"}`,
			"board":        `{"bob":"10","ann":"20"}`,
			"tags:1":       `["blue","green","red"]`,
		}
		for key, want := range wantValue {
			if values[key] != want {
				t.Errorf("%s = %s, want %s", key, values[key], want)
			}
		}
		if kinds["events"] != typeStream || !strings.Contains(values["events"], `"kind":"login"`) {
			t.Errorf("events = %s %s, want a stream holding the entry", kinds["events"], values["events"])
		}

		// The rows arrive in key order, which is what makes a page cut from them the same page twice.
		if !sortedByKey(rows.Rows) {
			t.Error("the rows are not in key order")
		}
	})

	t.Run("A clock is read in seconds and only where there is one", func(t *testing.T) {
		rows, err := live.TableRows(ctx, driver.TablePage{Namespace: ns, Table: keySpace})
		if err != nil {
			t.Fatalf("TableRows() error = %v", err)
		}

		for _, row := range rows.Rows {
			switch *row[0] {
			case "tmp:1":
				if row[3] == nil || *row[3] != "600" {
					t.Errorf("tmp:1 TTL = %v, want 600", row[3])
				}
			case "user:1":
				if row[3] != nil {
					t.Errorf("user:1 TTL = %v, want nothing", *row[3])
				}
			}
		}
	})

	t.Run("A filter on the key is answered by the server", func(t *testing.T) {
		rows, err := live.TableRows(ctx, driver.TablePage{
			Namespace: ns,
			Table:     keySpace,
			Filter:    andGroup(keyFilter(driver.FilterBegins, "user:")),
		})
		if err != nil {
			t.Fatalf("TableRows() error = %v", err)
		}
		if rows.Total != 2 {
			t.Errorf("Total = %d, want 2", rows.Total)
		}
		if !strings.Contains(rows.Statement, `MATCH user:*`) {
			t.Errorf("Statement = %q, want the glob in it", rows.Statement)
		}
	})

	t.Run("A filter on the type is answered here", func(t *testing.T) {
		rows, err := live.TableRows(ctx, driver.TablePage{
			Namespace: ns,
			Table:     keySpace,
			Filter: andGroup(driver.TableFilter{
				Column: columnType,
				Op:     driver.FilterEqual,
				Values: []string{typeString},
			}),
		})
		if err != nil {
			t.Fatalf("TableRows() error = %v", err)
		}
		if rows.Total != 3 {
			t.Errorf("Total = %d, want 3", rows.Total)
		}
	})

	t.Run("A page is cut from the ordered whole", func(t *testing.T) {
		first, err := live.TableRows(ctx, driver.TablePage{
			Namespace: ns, Table: keySpace, Limit: 3,
		})
		if err != nil {
			t.Fatalf("TableRows() error = %v", err)
		}
		second, err := live.TableRows(ctx, driver.TablePage{
			Namespace: ns, Table: keySpace, Limit: 3, Offset: 3,
		})
		if err != nil {
			t.Fatalf("TableRows() error = %v", err)
		}

		if len(first.Rows) != 3 || len(second.Rows) != 3 {
			t.Fatalf("pages = %d and %d rows, want 3 each", len(first.Rows), len(second.Rows))
		}
		if *first.Rows[2][0] >= *second.Rows[0][0] {
			t.Errorf("the second page starts at %s, which is not after %s",
				*second.Rows[0][0], *first.Rows[2][0])
		}
	})

	t.Run("A streamed read hands over every row", func(t *testing.T) {
		count := 0
		_, err := live.TableRows(ctx, driver.TablePage{
			Namespace: ns,
			Table:     keySpace,
			Stream:    &driver.RowStream{Rows: func(batch [][]*string) { count += len(batch) }},
		})
		if err != nil {
			t.Fatalf("TableRows() error = %v", err)
		}
		if count != 8 {
			t.Errorf("streamed %d rows, want 8", count)
		}
	})

	t.Run("Editing a value writes what the grid showed", func(t *testing.T) {
		edit := driver.CellEdit{
			Namespace: ns,
			Table:     keySpace,
			Column:    columnValue,
			Key:       map[string]string{columnKey: "session:4012"},
			Value:     cell(`{"domain":"10.0.0.1","port":"80","live":"yes"}`),
		}
		if err := live.UpdateCell(ctx, edit); err != nil {
			t.Fatalf("UpdateCell() error = %v", err)
		}

		reply := run(t, live, ns, "HGET session:4012 domain")
		if got := *reply.Rows[0][0]; got != "10.0.0.1" {
			t.Errorf("domain = %q, want 10.0.0.1", got)
		}
		reply = run(t, live, ns, "HLEN session:4012")
		if got := *reply.Rows[0][0]; got != "3" {
			t.Errorf("fields = %q, want 3", got)
		}
	})

	t.Run("Editing a value keeps the clock that was on it", func(t *testing.T) {
		edit := driver.CellEdit{
			Namespace: ns,
			Table:     keySpace,
			Column:    columnValue,
			Key:       map[string]string{columnKey: "tmp:1"},
			Value:     cell("still here"),
		}
		if err := live.UpdateCell(ctx, edit); err != nil {
			t.Fatalf("UpdateCell() error = %v", err)
		}

		reply := run(t, live, ns, "TTL tmp:1")
		if got := *reply.Rows[0][0]; got == "-1" {
			t.Error("the expiry was lost when the value was written")
		}
	})

	t.Run("A clock is set and taken off", func(t *testing.T) {
		edit := driver.CellEdit{
			Namespace: ns,
			Table:     keySpace,
			Column:    columnTTL,
			Key:       map[string]string{columnKey: "user:2"},
			Value:     cell("300"),
		}
		if err := live.UpdateCell(ctx, edit); err != nil {
			t.Fatalf("UpdateCell() error = %v", err)
		}
		if got := *run(t, live, ns, "TTL user:2").Rows[0][0]; got != "300" {
			t.Errorf("TTL = %q, want 300", got)
		}

		edit.Value = nil
		if err := live.UpdateCell(ctx, edit); err != nil {
			t.Fatalf("UpdateCell() error = %v", err)
		}
		if got := *run(t, live, ns, "TTL user:2").Rows[0][0]; got != "-1" {
			t.Errorf("TTL = %q, want it taken off", got)
		}
	})

	t.Run("A key is renamed, and never over another", func(t *testing.T) {
		edit := driver.CellEdit{
			Namespace: ns,
			Table:     keySpace,
			Column:    columnKey,
			Key:       map[string]string{columnKey: "user:2"},
			Value:     cell("user:3"),
		}
		if err := live.UpdateCell(ctx, edit); err != nil {
			t.Fatalf("UpdateCell() error = %v", err)
		}
		if got := *run(t, live, ns, "EXISTS user:3").Rows[0][0]; got != "1" {
			t.Errorf("EXISTS user:3 = %q, want 1", got)
		}

		edit.Key = map[string]string{columnKey: "user:3"}
		edit.Value = cell("user:1")
		if err := live.UpdateCell(ctx, edit); err == nil {
			t.Error("renaming over a key that is there was allowed")
		}
	})

	t.Run("A row is inserted and deleted", func(t *testing.T) {
		insert := driver.RowInsert{
			Namespace: ns,
			Table:     keySpace,
			Values: map[string]*string{
				columnKey:   cell("queue:sms"),
				columnType:  cell(typeList),
				columnValue: cell(`["one","two"]`),
				columnTTL:   cell("120"),
			},
		}
		if err := live.InsertRow(ctx, insert); err != nil {
			t.Fatalf("InsertRow() error = %v", err)
		}
		if got := *run(t, live, ns, "LRANGE queue:sms 0 -1").Rows[0][1]; got != "one" {
			t.Errorf("first element = %q, want one", got)
		}
		if got := *run(t, live, ns, "TTL queue:sms").Rows[0][0]; got != "120" {
			t.Errorf("TTL = %q, want 120", got)
		}

		if err := live.InsertRow(ctx, insert); err == nil {
			t.Error("inserting over a key that is there was allowed")
		}

		removal := driver.RowDelete{
			Namespace: ns,
			Table:     keySpace,
			Key:       map[string]string{columnKey: "queue:sms"},
		}
		if err := live.DeleteRow(ctx, removal); err != nil {
			t.Fatalf("DeleteRow() error = %v", err)
		}
		if got := *run(t, live, ns, "EXISTS queue:sms").Rows[0][0]; got != "0" {
			t.Errorf("EXISTS queue:sms = %q, want 0", got)
		}
	})

	t.Run("A search counts the keys it would open", func(t *testing.T) {
		matches, err := live.MatchingRows(ctx, driver.RowSearch{
			Namespace: ns,
			Table:     keySpace,
			Match:     domain.TextMatch{Text: "USER", Mode: domain.MatchContains, CaseInsensitive: true},
		})
		if err != nil {
			t.Fatalf("MatchingRows() error = %v", err)
		}
		if matches.Count != 2 {
			t.Errorf("Count = %d, want 2", matches.Count)
		}

		// The read it spells has to return exactly what it counted, which is the whole point of
		// answering with both.
		opened := run(t, live, ns, matches.Statement)
		if len(opened.Rows) != int(matches.Count) {
			t.Errorf("%q returned %d rows, want %d", matches.Statement, len(opened.Rows), matches.Count)
		}
	})

	t.Run("A search this engine cannot spell says so", func(t *testing.T) {
		_, err := live.MatchingRows(ctx, driver.RowSearch{
			Namespace: ns,
			Table:     keySpace,
			Match:     domain.TextMatch{Text: "^user", Mode: domain.MatchRegex},
		})
		if !errors.Is(err, domain.ErrUnsupported) {
			t.Errorf("MatchingRows() error = %v, want it unsupported", err)
		}
	})

	t.Run("A reply is drawn in the shape it has", func(t *testing.T) {
		if got := *run(t, live, ns, "PING").Rows[0][0]; got != "PONG" {
			t.Errorf("PING = %q, want PONG", got)
		}

		list := run(t, live, ns, "LRANGE queue:email 0 -1")
		if len(list.Rows) != 3 || *list.Rows[0][1] != "first" {
			t.Errorf("LRANGE = %v, want three rows", list.Rows)
		}

		pairs := run(t, live, ns, "CONFIG GET maxmemory")
		if len(pairs.Rows) != 1 || *pairs.Rows[0][0] != "maxmemory" {
			t.Errorf("CONFIG GET = %v, want one pair", pairs.Rows)
		}

		missing := run(t, live, ns, "GET nothing:here")
		if missing.Rows[0][0] != nil {
			t.Errorf("GET of a key that is not there = %v, want nothing", *missing.Rows[0][0])
		}
	})

	t.Run("A command that would take the connection away is refused", func(t *testing.T) {
		for _, command := range []string{"SELECT 0", "SUBSCRIBE news", "MULTI"} {
			_, err := live.Query(ctx, driver.Query{Namespace: ns, SQL: command})
			if !errors.Is(err, domain.ErrUnsupported) {
				t.Errorf("Query(%q) error = %v, want it unsupported", command, err)
			}
		}
	})

	t.Run("A rejection keeps the server's own wording", func(t *testing.T) {
		_, err := live.Query(ctx, driver.Query{Namespace: ns, SQL: "LPUSH user:1 x"})

		var rejected domain.StatementError
		if !errors.As(err, &rejected) {
			t.Fatalf("Query() error = %v, want a rejection", err)
		}
		if rejected.Code != "WRONGTYPE" {
			t.Errorf("Code = %q, want WRONGTYPE", rejected.Code)
		}
	})

	t.Run("What a key space has not got says which", func(t *testing.T) {
		refusals := map[string]error{
			"CreateTable":   live.CreateTable(ctx, driver.TableSpec{Namespace: ns, Table: "users"}),
			"DropTable":     live.DropTable(ctx, ns, keySpace),
			"CreateIndex":   live.CreateIndex(ctx, ns, keySpace, driver.IndexSpec{Name: "i"}),
			"AddColumn":     live.AddColumn(ctx, ns, keySpace, driver.ColumnSpec{Name: "c"}),
			"CreateDB":      live.CreateDatabase(ctx, driver.DatabaseSpec{Name: "3"}),
			"TruncateTable": live.TruncateTable(ctx, ns, keySpace),
		}
		for what, err := range refusals {
			if !errors.Is(err, domain.ErrUnsupported) {
				t.Errorf("%s error = %v, want it unsupported", what, err)
			}
		}
	})

	t.Run("A table that is not the key space is not there", func(t *testing.T) {
		_, err := live.TableRows(ctx, driver.TablePage{Namespace: ns, Table: "users"})
		if !errors.Is(err, domain.ErrUnknownTable) {
			t.Errorf("TableRows() error = %v, want the table missing", err)
		}
	})

	t.Run("Emptying takes every key", func(t *testing.T) {
		if err := live.EmptyTable(ctx, ns, keySpace); err != nil {
			t.Fatalf("EmptyTable() error = %v", err)
		}

		rows, err := live.TableRows(ctx, driver.TablePage{Namespace: ns, Table: keySpace})
		if err != nil {
			t.Fatalf("TableRows() error = %v", err)
		}
		if rows.Total != 0 {
			t.Errorf("Total = %d, want none left", rows.Total)
		}
	})
}

func sortedByKey(rows [][]*string) bool {
	for i := 1; i < len(rows); i++ {
		if *rows[i-1][0] > *rows[i][0] {
			return false
		}
	}
	return true
}
