// Package redis is the Redis adapter: a `driver.Driver` over go-redis.
//
// It is the second adapter that is not a `database/sql` one, and it differs from the first in a
// deeper place than MongoDB does. A document store still has collections to name and documents to
// read; a key-value store has neither. What it has is one flat key space per database, and three
// things follow from that:
//
//   - **The key space is the table.** There is exactly one, it is called `(All Data)`, and it holds
//     one row per key: the key, its type, its value, its time to live and what it costs. `keys.go`
//     reads it with `SCAN`, holds the names it found so that a page is cut from a stable order, and
//     fetches the values of one page at a time.
//   - **A value has a type rather than a column.** `value.go` reads and writes each of the six the
//     server has, spelling the five container types as **JSON** — so what the grid shows is exactly
//     what an edit hands back, which is the only way a value with structure can be edited in a cell.
//   - **A command is argv, not a sentence.** `command.go` rebuilds each argument from the tokens
//     `sqlkit.Lex` produces rather than scanning the text a second time, and what a command then
//     *does* is `domain.RedisHazard`'s rule and not this package's.
//
// Everything a key space has not got — a column, an index, a foreign key, a routine, a second table
// to name — is `domain.Unsupported` and says which, rather than being approximated. See
// docs/architecture.md.
package redis

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"sync"

	goredis "github.com/redis/go-redis/v9"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// keySpace is the one table a Redis database holds. It is named the way every other client names
// it, in brackets so it cannot collide with anything a reader would call a table of their own —
// there being no rule against a key called `keys`.
const keySpace = "(All Data)"

// defaultDatabases is how many a server is started with unless it was told otherwise, and is what
// this adapter falls back to when `CONFIG GET` is disabled — which is the usual arrangement on a
// hosted Redis. Guessing sixteen is right far more often than showing none.
const defaultDatabases = 16

// The adapter is held to both halves of the port at compile time, because the two are wide and a
// method missed here would otherwise only be missed where the registry is built.
var (
	_ driver.Driver  = (*Driver)(nil)
	_ driver.Session = (*session)(nil)
)

type Driver struct{}

func New() *Driver {
	return &Driver{}
}

func (*Driver) Engine() domain.Engine {
	return domain.EngineRedis
}

// Text is the lexical half of the engine, which needs no connection: a line is a command, a word is
// an argument and the verb is the first of them. See text.go.
func (*Driver) Text() driver.Text {
	return text{}
}

func (*Driver) Open(
	ctx context.Context,
	profile domain.ConnectionProfile,
	creds driver.Credentials,
	tunnel driver.Dialer,
) (driver.Session, error) {
	options, err := clientOptions(profile, creds, tunnel)
	if err != nil {
		return nil, err
	}

	home := 0
	if named := strings.TrimSpace(profile.Database); named != "" {
		// The profile is validated above this layer, so a database that will not parse cannot
		// arrive here — and if one did, database zero is the one every server has.
		home, _ = strconv.Atoi(named)
	}

	one := &session{options: options, home: home, clients: map[int]*goredis.Client{}}

	// Opening a client dials nothing: the pool is lazy, so a profile with the wrong host would
	// otherwise connect cleanly and fail on the first read.
	client, err := one.client(driver.Namespace{})
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx).Err(); err != nil {
		_ = one.Close()
		return nil, translateError(err)
	}

	return one, nil
}

// session is one connection to a server, and a client per database on it. Redis's `SELECT` is
// connection state rather than an argument, so a single pool would answer a read from whichever
// database the connection it picked happened to be on: a client per index is what makes a read of
// database 3 a read of database 3. It is the arrangement PostgreSQL's adapter has, for a
// different reason and with the same shape.
type session struct {
	options *goredis.Options
	// home is the database the profile named, and is where anything carrying no namespace goes.
	home int

	mu      sync.Mutex
	clients map[int]*goredis.Client
}

// client is the pool for the database a call is addressed to, opened the first time that database
// is read and closed with the session.
func (s *session) client(ns driver.Namespace) (*goredis.Client, error) {
	index, err := s.index(ns)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.clients[index]; ok {
		return existing, nil
	}

	options := *s.options
	options.DB = index
	client := goredis.NewClient(&options)
	s.clients[index] = client
	return client, nil
}

// index is the database a namespace names. A Redis database is numbered rather than named, so
// anything that is not a number is not one of them — and an empty namespace is the profile's own,
// which is what every call that is about the server rather than about a key carries.
func (s *session) index(ns driver.Namespace) (int, error) {
	name := strings.TrimSpace(ns.Database)
	if name == "" {
		return s.home, nil
	}

	index, err := strconv.Atoi(name)
	if err != nil || index < 0 {
		return 0, domain.ErrUnknownDatabase
	}
	return index, nil
}

func (s *session) Ping(ctx context.Context) error {
	client, err := s.client(driver.Namespace{})
	if err != nil {
		return err
	}
	return translateError(client.Ping(ctx).Err())
}

// ServerInfo reads the build out of `INFO server`, which is where Redis keeps every fact about
// itself that is not a key.
func (s *session) ServerInfo(ctx context.Context) (driver.ServerInfo, error) {
	client, err := s.client(driver.Namespace{})
	if err != nil {
		return driver.ServerInfo{}, err
	}

	info, err := client.Info(ctx, "server").Result()
	if err != nil {
		return driver.ServerInfo{}, translateError(err)
	}
	return driver.ServerInfo{Version: infoField(info, "redis_version")}, nil
}

// Databases lists the numbered databases the server was started with. They are not objects a reader
// creates — the set is fixed at start-up — so this is a count read out of `CONFIG GET databases`
// rather than a catalogue read.
//
// A hosted Redis usually refuses `CONFIG`, and a reader whose server refuses it still has databases:
// the fallback is the sixteen every default install has, together with any the keyspace itself
// reports and the one the profile named, so nothing the reader can actually reach goes unlisted.
func (s *session) Databases(ctx context.Context) ([]string, error) {
	client, err := s.client(driver.Namespace{})
	if err != nil {
		return nil, err
	}

	count := defaultDatabases
	if config, err := client.ConfigGet(ctx, "databases").Result(); err == nil {
		if parsed, err := strconv.Atoi(config["databases"]); err == nil && parsed > 0 {
			count = parsed
		}
	}

	reachable := map[int]bool{s.home: true}
	for index := 0; index < count; index++ {
		reachable[index] = true
	}
	if info, err := client.Info(ctx, "keyspace").Result(); err == nil {
		for index := range keyspaceCounts(info) {
			reachable[index] = true
		}
	}

	names := make([]string, 0, len(reachable))
	for index := range reachable {
		names = append(names, strconv.Itoa(index))
	}
	sort.Slice(names, func(a, b int) bool {
		left, _ := strconv.Atoi(names[a])
		right, _ := strconv.Atoi(names[b])
		return left < right
	})
	return names, nil
}

// Schemas has nothing to answer with: a Redis database holds its keys directly, so the tree stops
// at the level Databases already returned — the same shape MySQL and MongoDB have.
func (*session) Schemas(context.Context, string) ([]string, error) {
	return nil, nil
}

// Objects is the one folder a key space has, holding the one table there is. There is no view here
// and no routine: a Lua script is sent with the call that runs it and is not stored under a name
// the reader could list.
//
// The count comes with it because that list cannot carry it: one table always is one table, so what
// the tree shows beside a Redis database is `DBSIZE` — how many keys the database holds — and it is
// read here rather than in a second call so that opening the folder and being told its size are the
// same read, exactly as they are on an engine whose namespace is counted by what is listed.
func (s *session) Objects(ctx context.Context, ns driver.Namespace) (driver.Objects, error) {
	client, err := s.client(ns)
	if err != nil {
		return driver.Objects{}, err
	}

	size, err := client.DBSize(ctx).Result()
	if err != nil {
		return driver.Objects{}, translateError(err)
	}

	return driver.Objects{
		Tables:   []string{keySpace},
		Views:    []string{},
		Routines: []driver.Routine{},
		KeyCount: size,
	}, nil
}

// TableList is what the tree keeps about the key space: how many keys are in it and how many of
// those will expire. Both are the server's own numbers — `DBSIZE` and the line `INFO keyspace`
// keeps per database — rather than anything counted here.
func (s *session) TableList(ctx context.Context, ns driver.Namespace) (driver.TableList, error) {
	client, err := s.client(ns)
	if err != nil {
		return driver.TableList{}, err
	}
	index, err := s.index(ns)
	if err != nil {
		return driver.TableList{}, err
	}

	size, err := client.DBSize(ctx).Result()
	if err != nil {
		return driver.TableList{}, translateError(err)
	}

	expires := int64(0)
	if info, err := client.Info(ctx, "keyspace").Result(); err == nil {
		expires = keyspaceCounts(info)[index].expires
	}

	return driver.TableList{
		Columns: []driver.Column{
			{Name: "Name", Type: "string", FullType: "string"},
			{Name: "Keys", Type: "integer", FullType: "integer"},
			{Name: "Expiring", Type: "integer", FullType: "integer"},
		},
		Rows: [][]*string{{
			cell(keySpace), cell(strconv.FormatInt(size, 10)), cell(strconv.FormatInt(expires, 10)),
		}},
	}, nil
}

// TableSchemas is what the editor completes from: the one table and the columns every row of it
// has. They are the same five columns for every Redis server there has ever been, which is what a
// key space has instead of a catalogue.
func (*session) TableSchemas(context.Context, driver.Namespace) ([]driver.TableSchema, error) {
	names := make([]string, 0, len(keyColumns))
	for _, column := range keyColumns {
		names = append(names, column.Name)
	}
	return []driver.TableSchema{{Name: keySpace, Columns: names}}, nil
}

func (s *session) Close() error {
	s.mu.Lock()
	clients := s.clients
	s.clients = map[int]*goredis.Client{}
	s.mu.Unlock()

	var failure error
	for _, client := range clients {
		if err := client.Close(); err != nil && failure == nil {
			failure = translateError(err)
		}
	}
	return failure
}

// keyspaceCount is one line of `INFO keyspace`: `db0:keys=16922,expires=3,avg_ttl=0`.
type keyspaceCount struct {
	keys    int64
	expires int64
}

// keyspaceCounts reads that section into the databases it names. A database holding nothing has no
// line at all, which is exactly the zero this answers with.
func keyspaceCounts(info string) map[int]keyspaceCount {
	counts := map[int]keyspaceCount{}

	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		name, fields, found := strings.Cut(line, ":")
		if !found || !strings.HasPrefix(name, "db") {
			continue
		}
		index, err := strconv.Atoi(strings.TrimPrefix(name, "db"))
		if err != nil {
			continue
		}

		count := keyspaceCount{}
		for _, field := range strings.Split(fields, ",") {
			key, value, found := strings.Cut(field, "=")
			if !found {
				continue
			}
			number, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				continue
			}
			switch key {
			case "keys":
				count.keys = number
			case "expires":
				count.expires = number
			}
		}
		counts[index] = count
	}

	return counts
}

// infoField reads one `name:value` line out of an INFO section.
func infoField(info, name string) string {
	for _, line := range strings.Split(info, "\n") {
		field, value, found := strings.Cut(strings.TrimSpace(line), ":")
		if found && field == name {
			return value
		}
	}
	return ""
}

// cell is a value on its way into a row. Every column of a key space holds one, and the only NULL
// among them is a key with no expiry — see keys.go.
func cell(value string) *string {
	return &value
}
