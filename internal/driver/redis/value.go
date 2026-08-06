package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	goredis "github.com/redis/go-redis/v9"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

// The six types a key can be. They are the server's own spellings, because `TYPE` answers with
// these and a reader who knows Redis knows these words — the same rule the MongoDB adapter follows
// with `objectId`.
const (
	typeString = "string"
	typeList   = "list"
	typeSet    = "set"
	typeHash   = "hash"
	typeZSet   = "zset"
	typeStream = "stream"
	// typeMissing is what the server answers for a key that is not there, which is what a key read
	// after it expired looks like.
	typeMissing = "none"
)

const (
	// previewItems is how many members of a container a grid row carries. It is a *sample* and says
	// so — see sampled — for the reason mongo.fields is one: a list of a million elements is not
	// something a page of a hundred rows can hold, and pretending otherwise would freeze the window
	// rather than fill it.
	previewItems = 50
	// previewChars is the same bound for a string, which has no members to count. Eight thousand is
	// far more than a grid row shows and far less than a value that would cost the reader the page.
	previewChars = 8192
)

// sampleMark is what a value the reader is only being shown part of ends with. It is deliberately
// not valid JSON and deliberately not the value: an edit of a sampled value would write away
// whatever was not shown, so the mark is what makes that edit fail — see parseValue.
const sampleMark = "…"

// sampled says how much of a value the reader is looking at, and answers the value itself when that
// is all of it.
func sampled(text string, shown, total int64) string {
	if shown >= total {
		return text
	}
	return fmt.Sprintf("%s %s %d of %d", text, sampleMark, shown, total)
}

// valueRead is one key's value in flight: the commands that read it, queued in a pipeline with
// every other key of the page, and the type that says how to read their replies back.
//
// Two commands and never more, so a page of a hundred keys is two round trips rather than two
// hundred: how many members there are, and the first previewItems of them.
type valueRead struct {
	kind  string
	total *goredis.IntCmd
	items goredis.Cmder
}

// queueValue asks a pipeline for one key's value. A type this adapter does not know — a module's,
// which is what `MBbloom--` and `TSDB-TYPE` are — queues nothing and reads as its own name, because
// a module's value has no general shape and inventing one would be inventing the data.
func queueValue(ctx context.Context, pipe goredis.Pipeliner, key, kind string) valueRead {
	read := valueRead{kind: kind}

	switch kind {
	case typeString:
		read.total = pipe.StrLen(ctx, key)
		read.items = pipe.GetRange(ctx, key, 0, previewChars-1)
	case typeList:
		read.total = pipe.LLen(ctx, key)
		read.items = pipe.LRange(ctx, key, 0, previewItems-1)
	case typeSet:
		read.total = pipe.SCard(ctx, key)
		read.items = pipe.SScan(ctx, key, 0, "", previewItems)
	case typeHash:
		read.total = pipe.HLen(ctx, key)
		read.items = pipe.HScan(ctx, key, 0, "", previewItems)
	case typeZSet:
		read.total = pipe.ZCard(ctx, key)
		read.items = pipe.ZRangeWithScores(ctx, key, 0, previewItems-1)
	case typeStream:
		read.total = pipe.XLen(ctx, key)
		read.items = pipe.XRangeN(ctx, key, "-", "+", previewItems)
	}

	return read
}

// value reads what the pipeline came back with. A string is itself and the five containers are
// **JSON**, which is the whole reason a value with structure can be edited in a cell at all: what
// the grid draws is exactly what an edit hands back, and there is no second notation to learn.
//
// A member is written as a string even where it reads as a number, because that is what it is: Redis
// stores bytes, and a hash field holding `36` is the two characters and not the integer.
func (r valueRead) value() (string, error) {
	if r.items == nil {
		return "(" + r.kind + ")", nil
	}
	if err := r.items.Err(); err != nil && !isNil(err) {
		return "", translateError(err)
	}

	total := int64(0)
	if r.total != nil {
		total = r.total.Val()
	}

	switch items := r.items.(type) {
	case *goredis.StringCmd:
		return sampled(items.Val(), int64(len(items.Val())), total), nil

	case *goredis.StringSliceCmd:
		members := items.Val()
		return sampled(jsonArray(members), int64(len(members)), total), nil

	case *goredis.ScanCmd:
		found, _ := items.Val()
		if r.kind == typeHash {
			return sampled(jsonPairs(hashPairs(found)), int64(len(found)/2), total), nil
		}
		// A set has no order of its own, so it is shown in one: an arbitrary order would redraw the
		// same value differently every time the page was read.
		sort.Strings(found)
		return sampled(jsonArray(found), int64(len(found)), total), nil

	case *goredis.ZSliceCmd:
		members := items.Val()
		pairs := make([][2]string, 0, len(members))
		for _, member := range members {
			name, _ := member.Member.(string)
			pairs = append(pairs, [2]string{name, strconv.FormatFloat(member.Score, 'f', -1, 64)})
		}
		return sampled(jsonPairs(pairs), int64(len(members)), total), nil

	case *goredis.XMessageSliceCmd:
		return sampled(jsonStream(items.Val()), int64(len(items.Val())), total), nil
	}

	return "", nil
}

// hashPairs reads HSCAN's flat reply — a field, then its value, then the next field — as the pairs
// it is, in field order so that reading the same hash twice draws it the same way twice.
func hashPairs(flat []string) [][2]string {
	pairs := make([][2]string, 0, len(flat)/2)
	for i := 0; i+1 < len(flat); i += 2 {
		pairs = append(pairs, [2]string{flat[i], flat[i+1]})
	}
	sort.Slice(pairs, func(a, b int) bool { return pairs[a][0] < pairs[b][0] })
	return pairs
}

// jsonArray, jsonPairs and jsonStream are written by hand rather than marshalled from a map for one
// reason: **order**. A sorted set is its rank order and a hash is drawn in field order, and a Go map
// has neither. Each string still goes through the encoder, so the escaping is the encoder's.
func jsonArray(items []string) string {
	var out strings.Builder
	out.WriteByte('[')
	for i, item := range items {
		if i > 0 {
			out.WriteByte(',')
		}
		out.WriteString(quoteJSON(item))
	}
	out.WriteByte(']')
	return out.String()
}

func jsonPairs(pairs [][2]string) string {
	var out strings.Builder
	out.WriteByte('{')
	for i, pair := range pairs {
		if i > 0 {
			out.WriteByte(',')
		}
		out.WriteString(quoteJSON(pair[0]))
		out.WriteByte(':')
		out.WriteString(quoteJSON(pair[1]))
	}
	out.WriteByte('}')
	return out.String()
}

// jsonStream writes an entry as its id and its fields, which is the one type here whose shape is not
// the reader's to choose: an id is the server's and a stream is append-only, so this is read and
// never written back — see writeValue.
func jsonStream(messages []goredis.XMessage) string {
	var out strings.Builder
	out.WriteByte('[')
	for i, message := range messages {
		if i > 0 {
			out.WriteByte(',')
		}

		fields := make([][2]string, 0, len(message.Values))
		for name, value := range message.Values {
			fields = append(fields, [2]string{name, fmt.Sprint(value)})
		}
		sort.Slice(fields, func(a, b int) bool { return fields[a][0] < fields[b][0] })

		out.WriteString(`{"id":`)
		out.WriteString(quoteJSON(message.ID))
		out.WriteString(`,"fields":`)
		out.WriteString(jsonPairs(fields))
		out.WriteByte('}')
	}
	out.WriteByte(']')
	return out.String()
}

func quoteJSON(text string) string {
	encoded, err := json.Marshal(text)
	if err != nil {
		return `""`
	}
	return string(encoded)
}

// writeValue replaces what a key holds, keeping whatever expiry it had: an edit to a value is an
// edit to the value and not to the clock on it.
//
// A container is written whole — cleared, then filled from the JSON the reader handed back — because
// that is what editing a cell *means*. It goes through a transaction so that a key is never seen
// empty by anything else, which a DEL and a fill spread over two calls could not promise.
func writeValue(ctx context.Context, client *goredis.Client, key, kind, text string) error {
	if strings.Contains(text, sampleMark) {
		return domain.StatementError{
			Code: "SAMPLE",
			Message: "this value is a sample of a larger one; writing it back would drop " +
				"whatever the grid did not show",
		}
	}

	if kind == typeString {
		// KEEPTTL is the server's own answer to the whole problem, so a string needs no transaction
		// and no clock arithmetic.
		return translateError(client.Do(ctx, "SET", key, text, "KEEPTTL").Err())
	}

	members, err := parseValue(kind, text)
	if err != nil {
		return err
	}

	ttl, err := client.PTTL(ctx, key).Result()
	if err != nil && !isNil(err) {
		return translateError(err)
	}

	_, err = client.TxPipelined(ctx, func(pipe goredis.Pipeliner) error {
		pipe.Del(ctx, key)
		for _, one := range members {
			sent := make([]any, 0, len(one.args)+2)
			sent = append(sent, one.name, key)
			for _, argument := range one.args {
				sent = append(sent, argument)
			}
			pipe.Do(ctx, sent...)
		}
		// A negative PTTL is a key with no expiry (-1) or no key at all (-2); neither is a clock to
		// put back.
		if milliseconds := ttl.Milliseconds(); milliseconds > 0 {
			pipe.PExpire(ctx, key, ttl)
		}
		return nil
	})
	return translateError(err)
}

// write is one command a value is rebuilt by. There is more than one per value because a hash of two
// hundred fields is not one HSET a server will take, and because a list is filled in order.
type write struct {
	name string
	args []string
}

// batchArgs is how many members one rebuild command carries. It is the same bargain
// `driver.RowWrite.Batch` strikes: enough to make a rebuild a handful of commands rather than a
// thousand, and short enough that no server refuses the line.
const batchArgs = 200

// parseValue reads a container's JSON back into the commands that would rebuild it. It is strict on
// purpose — a value that will not parse is refused rather than half-written — and the wording it
// refuses with is the reader's own mistake spelled out, because nothing was sent anywhere and the
// message is the only useful thing there is to show. That is the code `SHELL` position exactly; see
// docs/errors.md.
func parseValue(kind, text string) ([]write, error) {
	switch kind {
	case typeList, typeSet:
		items, err := parseArray(text)
		if err != nil {
			return nil, err
		}
		name := "RPUSH"
		if kind == typeSet {
			name = "SADD"
		}
		return batched(name, items), nil

	case typeHash:
		pairs, err := parseObject(text)
		if err != nil {
			return nil, err
		}
		flat := make([]string, 0, len(pairs)*2)
		for _, pair := range pairs {
			flat = append(flat, pair[0], pair[1])
		}
		return batched("HSET", flat), nil

	case typeZSet:
		pairs, err := parseObject(text)
		if err != nil {
			return nil, err
		}
		flat := make([]string, 0, len(pairs)*2)
		for _, pair := range pairs {
			if _, err := strconv.ParseFloat(pair[1], 64); err != nil {
				return nil, valueError("the score of " + pair[0] + " is not a number")
			}
			// ZADD takes the score first, which is the one place a pair is written the other way
			// round from how it is read.
			flat = append(flat, pair[1], pair[0])
		}
		return batched("ZADD", flat), nil

	case typeStream:
		return nil, domain.Unsupported("editing a stream")
	}

	return nil, domain.Unsupported("editing a " + kind + " value")
}

// batched cuts a rebuild into commands no server will refuse for length. An empty container is one
// no command at all: a DEL has already emptied it, and a key holding nothing does not exist here.
func batched(name string, args []string) []write {
	writes := []write{}
	for start := 0; start < len(args); start += batchArgs {
		end := min(start+batchArgs, len(args))
		writes = append(writes, write{name: name, args: args[start:end]})
	}
	return writes
}

func parseArray(text string) ([]string, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return nil, valueError("is not a JSON array")
	}

	items := make([]string, 0, len(raw))
	for _, one := range raw {
		item, err := scalar(one)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// parseObject reads a JSON object as the pairs it holds, in the order it holds them: a sorted set is
// its order, and rebuilding one from a Go map would put its members back in whatever order the
// runtime felt like.
func parseObject(text string) ([][2]string, error) {
	decoder := json.NewDecoder(strings.NewReader(text))

	open, err := decoder.Token()
	if err != nil || open != json.Delim('{') {
		return nil, valueError("is not a JSON object")
	}

	pairs := [][2]string{}
	for decoder.More() {
		name, err := decoder.Token()
		if err != nil {
			return nil, valueError("is not a JSON object")
		}
		field, ok := name.(string)
		if !ok {
			return nil, valueError("is not a JSON object")
		}

		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return nil, valueError("is not a JSON object")
		}
		value, err := scalar(raw)
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, [2]string{field, value})
	}
	return pairs, nil
}

// scalar reads one member. A string is itself and a number, a boolean or a null is the text of one,
// because that is what the server would store either way: `{"n": 1}` and `{"n": "1"}` are the same
// hash, so refusing the first would be refusing something Redis has no opinion about.
func scalar(raw json.RawMessage) (string, error) {
	trimmed := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trimmed, `"`) {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return "", valueError("holds a string this app cannot read")
		}
		return text, nil
	}
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return "", valueError("holds a nested value, and a Redis member is a string")
	}
	if trimmed == "null" {
		return "", nil
	}
	return trimmed, nil
}

func valueError(what string) error {
	return domain.StatementError{Code: "VALUE", Message: "the value " + what}
}

// isNil says the server answered that there is nothing there, which is not a failure: a GET of a key
// that has gone, a TTL of one that never had a clock. Every read here answers it as the fact it is.
func isNil(err error) bool {
	return errors.Is(err, goredis.Nil)
}
