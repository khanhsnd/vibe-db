package redis

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	goredis "github.com/redis/go-redis/v9"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// The five columns every row of a key space has. They are not read from anywhere: a key space
// declares nothing, so unlike a document store — where the fields are *inferred* from what the
// documents turned out to hold — these are the same five on every Redis server there has ever been.
const (
	columnKey   = "Key"
	columnType  = "Type"
	columnValue = "Value"
	columnTTL   = "TTL"
	columnSize  = "Size"
)

// keyColumns is that shape, and the key is the primary key: a Redis key is the only thing that
// identifies a row here, and it identifies one exactly — which is what makes a cell editable and a
// row deletable without anything else being asked of the reader.
var keyColumns = []driver.Column{
	{Name: columnKey, Type: "key", FullType: "key", PrimaryKey: true},
	{Name: columnType, Type: "type", FullType: "type"},
	{Name: columnValue, Type: "value", FullType: "value", Nullable: true},
	{Name: columnTTL, Type: "seconds", FullType: "seconds", Nullable: true},
	{Name: columnSize, Type: "bytes", FullType: "bytes", Nullable: true},
}

const (
	// scanBatch is the COUNT hint one SCAN carries. It is a hint and not a promise — the server
	// answers with whatever a bucket held — which is the whole reason a scan is a loop.
	scanBatch = 1000
	// scanCap is how many keys a read will hold names for before it refuses. A page is cut from a
	// *sorted* list, so the names have to be held all at once, and a database of millions is one
	// nobody browses whole: the reader is told to narrow it rather than shown a window that lies
	// about how big it is.
	scanCap = 1_000_000
	// attributeChunk and valueChunk are how many keys one pipeline asks about. Three commands a key
	// for its type, its clock and its cost, and two for its value.
	attributeChunk = 500
	valueChunk     = 200
	// filterValueCap is how many keys may be *read* to answer a filter on the value column. A filter
	// on a key's name costs a comparison and a filter on its value costs the value, which is why one
	// is unbounded and the other is not.
	filterValueCap = 5000
	// streamBatch is how many rows a streamed read hands over at a time — see driver.RowStream.
	streamBatch = 500
)

// keyRow is one key as the grid draws it: what it is called, what it holds, when it expires and what
// it costs. `valued` says the value was actually read, because a filter on the name alone never
// reads one.
type keyRow struct {
	name   string
	kind   string
	ttl    int64
	size   int64
	value  string
	valued bool
}

// noExpiry and noSize are the two absences a row can carry: a key nobody put a clock on, and a
// server that will not say what a key costs — `MEMORY USAGE` is not in every build, and neither is
// worth failing a read of ten thousand keys over.
const (
	noExpiry = int64(-1)
	noSize   = int64(-1)
)

func (r keyRow) row() []*string {
	return []*string{
		cell(r.name),
		cell(r.kind),
		optional(r.valued, r.value),
		seconds(r.ttl),
		number(r.size),
	}
}

// seconds is a clock as the reader sets one. Redis keeps milliseconds and a reader thinks in
// seconds — `EXPIRE key 86` is what they typed — so the row rounds up: half a second left is a key
// that has not expired yet, and a `0` there would read as one that has.
func seconds(milliseconds int64) *string {
	if milliseconds < 0 {
		return nil
	}
	return cell(strconv.FormatInt((milliseconds+999)/1000, 10))
}

func number(value int64) *string {
	if value < 0 {
		return nil
	}
	return cell(strconv.FormatInt(value, 10))
}

func optional(present bool, value string) *string {
	if !present {
		return nil
	}
	return cell(value)
}

// isKeySpace says this is the one table a Redis database has. Anything else is a table that does not
// exist, which is the honest answer: there is nowhere for it to be.
func isKeySpace(table string) bool {
	return strings.EqualFold(strings.TrimSpace(table), keySpace)
}

// TableStructure is the shape of the key space, which is a constant. There are no indexes — a Redis
// key *is* the index — and no constraints of any kind, so the two lists are empty rather than
// refused: a table with no indexes is an ordinary thing to be, and `Unsupported` is for a concept
// the engine has not got rather than for a list that happens to be empty.
func (s *session) TableStructure(
	ctx context.Context,
	ns driver.Namespace,
	table string,
) (driver.TableStructure, error) {
	if _, err := s.client(ns); err != nil {
		return driver.TableStructure{}, err
	}
	if !isKeySpace(table) {
		return driver.TableStructure{}, domain.ErrUnknownTable
	}

	return driver.TableStructure{
		Columns:     append([]driver.Column(nil), keyColumns...),
		Indexes:     []driver.Index{},
		ForeignKeys: []driver.ForeignKey{},
	}, nil
}

// TableRows is one page of the key space.
//
// It is three steps and the order of them is the whole design. **The names first**, narrowed by a
// glob wherever the reader's filter can be spelled as one, because that is the only narrowing a
// Redis server will do for us. **Then whatever the filter and the sort actually need** — a type, a
// clock, a cost — for every key that survived, and nothing else: a read filtered by name alone never
// asks the server what any of those keys hold. **Then the page**, whose values are read last and for
// one page only, which is what keeps a grid of a hundred rows two round trips rather than a hundred.
func (s *session) TableRows(ctx context.Context, page driver.TablePage) (driver.TableRows, error) {
	client, err := s.client(page.Namespace)
	if err != nil {
		return driver.TableRows{}, err
	}
	if !isKeySpace(page.Table) {
		return driver.TableRows{}, domain.ErrUnknownTable
	}

	wanted, err := wants(page.Filter, page.Sort)
	if err != nil {
		return driver.TableRows{}, err
	}

	names, err := scanKeys(ctx, client, glob(page.Filter))
	if err != nil {
		return driver.TableRows{}, err
	}
	sort.Strings(names)

	rows, err := narrow(ctx, client, names, page.Filter, wanted)
	if err != nil {
		return driver.TableRows{}, err
	}
	if err := order(rows, page.Sort); err != nil {
		return driver.TableRows{}, err
	}

	answer := driver.TableRows{
		Columns:   append([]driver.Column(nil), keyColumns...),
		Total:     int64(len(rows)),
		Statement: scanStatement(glob(page.Filter)),
	}

	if page.Stream != nil {
		return answer, s.streamRows(ctx, client, rows, page)
	}

	window := cut(rows, page.Offset, page.Limit)
	filled, err := fill(ctx, client, window)
	if err != nil {
		return driver.TableRows{}, err
	}

	answer.Rows = make([][]*string, 0, len(filled))
	for _, one := range filled {
		answer.Rows = append(answer.Rows, one.row())
	}
	return answer, nil
}

// streamRows hands a read of every key over batch by batch, so a key space too large to answer in
// one piece costs one batch of memory rather than a database's worth — the arrangement every other
// adapter's streamed read has, and what an export of a whole key space is built on.
func (s *session) streamRows(
	ctx context.Context,
	client *goredis.Client,
	rows []keyRow,
	page driver.TablePage,
) error {
	window := cut(rows, page.Offset, page.Limit)

	for start := 0; start < len(window); start += streamBatch {
		filled, err := fill(ctx, client, window[start:min(start+streamBatch, len(window))])
		if err != nil {
			return err
		}

		batch := make([][]*string, 0, len(filled))
		for _, one := range filled {
			batch = append(batch, one.row())
		}
		page.Stream.Rows(batch)
		if page.Stream.Counted != nil {
			page.Stream.Counted(start + len(batch))
		}
	}
	return nil
}

// cut is the window a page asks for. A limit of zero or less is every row, which is the convention
// TablePage carries and what a streamed read of a whole key space uses.
func cut(rows []keyRow, offset, limit int) []keyRow {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(rows) {
		return nil
	}
	rows = rows[offset:]
	if limit > 0 && limit < len(rows) {
		rows = rows[:limit]
	}
	return rows
}

// scanKeys walks the whole key space, or the part of it a glob admits. It is a loop because SCAN is
// a cursor: the server hands back a bucket at a time and promises only that a key present for the
// whole walk is seen at least once, which is why the names are deduplicated by the sort that follows
// rather than trusted to arrive once.
func scanKeys(ctx context.Context, client *goredis.Client, pattern string) ([]string, error) {
	names := []string{}
	cursor := uint64(0)

	for {
		found, next, err := client.Scan(ctx, cursor, pattern, scanBatch).Result()
		if err != nil {
			return nil, translateError(err)
		}
		names = append(names, found...)

		if len(names) > scanCap {
			return nil, domain.StatementError{
				Code: "SCANLIMIT",
				Message: fmt.Sprintf(
					"this database holds more than %d keys; narrow the read with a filter on %s",
					scanCap, columnKey,
				),
			}
		}

		cursor = next
		if cursor == 0 {
			return names, nil
		}
	}
}

// scanStatement is the read the reader is shown, and it is the read that ran: the same glob, the
// same batch. What it does not carry is the page, there being no offset in a cursor — which is why
// the grid's own count is the honest place to read how far in a page sits.
func scanStatement(pattern string) string {
	return fmt.Sprintf("SCAN 0 MATCH %s COUNT %d", quoteArgument(pattern), scanBatch)
}

// narrow applies the reader's filter to the keys the scan found. A filter naming only the key is
// answered from the names alone; one naming a type, a clock or a cost reads those three for every
// candidate; and one naming the *value* reads values, which is the one narrowing that is bounded —
// see filterValueCap.
func narrow(
	ctx context.Context,
	client *goredis.Client,
	names []string,
	filter driver.TableGroup,
	wanted columns,
) ([]keyRow, error) {
	rows := make([]keyRow, 0, len(names))
	for _, name := range names {
		rows = append(rows, keyRow{name: name, ttl: noExpiry, size: noSize})
	}

	if len(filter.Children) == 0 && !wanted.beyondName() {
		return rows, nil
	}

	if wanted.beyondName() {
		if wanted[columnValue] && len(rows) > filterValueCap {
			return nil, domain.StatementError{
				Code: "SCANLIMIT",
				Message: fmt.Sprintf(
					"reading the %s of %d keys is more than this will do; narrow the read with a "+
						"filter on %s first, which costs the server nothing",
					columnValue, len(rows), columnKey,
				),
			}
		}

		read, err := readRows(ctx, client, rows, wanted[columnValue])
		if err != nil {
			return nil, err
		}
		rows = read
	}

	kept := make([]keyRow, 0, len(rows))
	for _, row := range rows {
		ok, err := matches(row, filter)
		if err != nil {
			return nil, err
		}
		if ok {
			kept = append(kept, row)
		}
	}
	return kept, nil
}

// fill reads the values of the rows the reader is about to see, and nothing else's.
func fill(ctx context.Context, client *goredis.Client, rows []keyRow) ([]keyRow, error) {
	for _, row := range rows {
		if !row.valued {
			return readRows(ctx, client, rows, true)
		}
	}
	return rows, nil
}

// readRows asks the server about a list of keys, in two pipelines and never more: the first for what
// each key *is* — its type, its clock and its cost — and the second, which needs the first's answer
// to know what to ask, for what it holds.
//
// A command that fails for one key does not fail the read. A key that expired between the scan and
// this reads as a type of `none`, and a server with no `MEMORY USAGE` reads as a cost of nothing;
// neither is worth refusing a page of a hundred rows over.
func readRows(
	ctx context.Context,
	client *goredis.Client,
	rows []keyRow,
	withValue bool,
) ([]keyRow, error) {
	filled := append([]keyRow(nil), rows...)

	for start := 0; start < len(filled); start += attributeChunk {
		chunk := filled[start:min(start+attributeChunk, len(filled))]

		pipe := client.Pipeline()
		kinds := make([]*goredis.StatusCmd, len(chunk))
		clocks := make([]*goredis.DurationCmd, len(chunk))
		costs := make([]*goredis.IntCmd, len(chunk))
		for i, row := range chunk {
			kinds[i] = pipe.Type(ctx, row.name)
			clocks[i] = pipe.PTTL(ctx, row.name)
			costs[i] = pipe.MemoryUsage(ctx, row.name)
		}
		if err := exec(ctx, pipe); err != nil {
			return nil, err
		}

		for i := range chunk {
			chunk[i].kind = kinds[i].Val()
			chunk[i].ttl = clockOf(clocks[i])
			chunk[i].size = noSize
			if cost, err := costs[i].Result(); err == nil {
				chunk[i].size = cost
			}
		}
	}

	if !withValue {
		return filled, nil
	}

	for start := 0; start < len(filled); start += valueChunk {
		chunk := filled[start:min(start+valueChunk, len(filled))]

		pipe := client.Pipeline()
		reads := make([]valueRead, len(chunk))
		for i, row := range chunk {
			reads[i] = queueValue(ctx, pipe, row.name, row.kind)
		}
		if err := exec(ctx, pipe); err != nil {
			return nil, err
		}

		for i := range chunk {
			value, err := reads[i].value()
			if err != nil {
				return nil, err
			}
			chunk[i].value, chunk[i].valued = value, true
		}
	}

	return filled, nil
}

// exec runs a pipeline and answers only for the failures that are the *connection's*. A rejection of
// one command in it — a key that has gone, a command this server does not have — is read where that
// command's reply is read, because a page of a hundred keys must not be lost to one of them.
func exec(ctx context.Context, pipe goredis.Pipeliner) error {
	if _, err := pipe.Exec(ctx); err != nil && !isNil(err) {
		if _, rejected := translateError(err).(domain.StatementError); rejected {
			return nil
		}
		return translateError(err)
	}
	return nil
}

// clockOf reads a PTTL. The server answers -1 for a key with no expiry and -2 for one that is not
// there, and go-redis hands both back as those numbers of *nanoseconds* — which is why this reads
// the sign rather than the duration.
func clockOf(clock *goredis.DurationCmd) int64 {
	value, err := clock.Result()
	if err != nil || value < 0 {
		return noExpiry
	}
	return value.Milliseconds()
}

// ColumnValues lists what a column of the key space actually holds, for a reader picking a filter's
// value rather than typing it. A type is the six the server has, which is a list that needs no read
// at all; anything else is read from the keys themselves and capped, because a column of a million
// rows is not a list anybody scrolls.
func (s *session) ColumnValues(ctx context.Context, pick driver.ColumnPick) ([]string, error) {
	client, err := s.client(pick.Namespace)
	if err != nil {
		return nil, err
	}
	if !isKeySpace(pick.Table) {
		return nil, domain.ErrUnknownTable
	}

	if strings.EqualFold(pick.Column, columnType) {
		return []string{typeHash, typeList, typeSet, typeStream, typeString, typeZSet}, nil
	}
	if !strings.EqualFold(pick.Column, columnKey) {
		return nil, domain.Unsupported("picking a value from " + pick.Column)
	}

	names, err := scanKeys(ctx, client, "*")
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	if pick.Limit > 0 && len(names) > pick.Limit {
		names = names[:pick.Limit]
	}
	return names, nil
}
