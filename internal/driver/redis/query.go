package redis

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// The columns a reply is drawn in. A Redis reply has no names of its own — it is a value, a list of
// values or a list of lists — so these are the app's, and they are three because that is how many
// shapes there are.
const (
	replyLabel = "Reply"
	indexLabel = "#"
	fieldLabel = "Field"
	valueLabel = "Value"
)

// blocked are the commands this tab will not run, and each of them for a reason the reader cannot
// see coming.
//
// `SELECT` and `SWAPDB` change which database the *connection* is on, and a connection here is one
// of a pool shared by every read of that database: a tab that ran one would leave the grid reading
// from somewhere else, at a moment nothing could predict. The database a tab is open on is the one
// it uses, and changing it is done by opening another.
//
// The rest never answer. `SUBSCRIBE` and `MONITOR` put the connection into a mode where it only
// listens, and the pool would wait on that connection for as long as the app was open.
var blocked = map[string]string{
	"select":      "the database a tab reads is the one it was opened on",
	"swapdb":      "the database a tab reads is the one it was opened on",
	"subscribe":   "a subscription never answers, and would hold the connection open",
	"psubscribe":  "a subscription never answers, and would hold the connection open",
	"ssubscribe":  "a subscription never answers, and would hold the connection open",
	"monitor":     "a monitor never answers, and would hold the connection open",
	"reset":       "the connection is the pool's rather than the tab's",
	"hello":       "the protocol is settled when the connection is made",
	"auth":        "the credentials are the profile's",
	"watch":       "a transaction cannot span two commands here: each is sent on its own connection",
	"multi":       "a transaction cannot span two commands here: each is sent on its own connection",
	"exec":        "a transaction cannot span two commands here: each is sent on its own connection",
	"discard":     "a transaction cannot span two commands here: each is sent on its own connection",
	"unsubscribe": "there is no subscription on this connection to end",
}

// Query runs one command and draws what came back.
//
// It is *one* command and not a script: the app splits a script above this — see text.Split, where a
// newline ends a command — so anything arriving here with two lines in it is one command holding a
// newline, which is a value and not a second command.
func (s *session) Query(ctx context.Context, query driver.Query) (driver.QueryResult, error) {
	client, err := s.client(query.Namespace)
	if err != nil {
		return driver.QueryResult{}, err
	}
	if query.Explain {
		// Redis has no plan to ask for. Every command's cost is in its own documentation and its
		// complexity is fixed, so there is nothing here that `EXPLAIN` would answer.
		return driver.QueryResult{}, domain.Unsupported("explaining a Redis command")
	}

	args, err := arguments(query.SQL)
	if err != nil {
		return driver.QueryResult{}, err
	}
	if len(args) == 0 {
		return driver.QueryResult{}, domain.ErrEmptyStatement
	}
	if reason, refused := blocked[strings.ToLower(args[0])]; refused {
		return driver.QueryResult{}, domain.Unsupported(strings.ToUpper(args[0]) + ": " + reason)
	}

	sent := make([]any, 0, len(args))
	for _, one := range args {
		sent = append(sent, one)
	}

	reply, err := client.Do(ctx, sent...).Result()
	if err != nil && !isNil(err) {
		return driver.QueryResult{}, translateError(err)
	}

	columns, rows := render(reply, isNil(err))
	result := driver.QueryResult{Columns: columns, Rows: rows}
	if query.MaxRows > 0 && len(result.Rows) > query.MaxRows {
		result.Rows = result.Rows[:query.MaxRows]
		result.Truncated = true
	}
	return result, nil
}

// render draws a reply in whichever of the three shapes it has: one value, a list of values, or the
// pairs a RESP3 map arrives as. A list of lists — which is what `XRANGE` and a scan answer with — is
// drawn as a column per position, because the alternative is one cell holding a whole nested reply
// and nothing to read in it.
func render(reply any, missing bool) ([]driver.Column, [][]*string) {
	if missing {
		// The server said there is nothing there, which is an answer rather than an absence of one.
		return []driver.Column{replyColumn(replyLabel)}, [][]*string{{nil}}
	}

	switch value := reply.(type) {
	case []any:
		return renderList(value)
	case map[any]any:
		return renderMap(value)
	default:
		return []driver.Column{replyColumn(replyLabel)}, [][]*string{{replyCell(reply)}}
	}
}

func renderList(items []any) ([]driver.Column, [][]*string) {
	widest := 0
	for _, item := range items {
		if nested, ok := item.([]any); ok {
			widest = max(widest, len(nested))
		}
	}

	if widest == 0 {
		columns := []driver.Column{replyColumn(indexLabel), replyColumn(valueLabel)}
		rows := make([][]*string, 0, len(items))
		for i, item := range items {
			rows = append(rows, []*string{cell(strconv.Itoa(i + 1)), replyCell(item)})
		}
		return columns, rows
	}

	columns := []driver.Column{replyColumn(indexLabel)}
	for i := 0; i < widest; i++ {
		columns = append(columns, replyColumn(strconv.Itoa(i+1)))
	}

	rows := make([][]*string, 0, len(items))
	for i, item := range items {
		row := make([]*string, widest+1)
		row[0] = cell(strconv.Itoa(i + 1))

		nested, ok := item.([]any)
		if !ok {
			// A list that mixes a value with a list of them — which is what a scan's cursor makes it
			// — puts the odd one in the first column rather than nowhere.
			row[1] = replyCell(item)
			rows = append(rows, row)
			continue
		}
		for at, one := range nested {
			row[at+1] = replyCell(one)
		}
		rows = append(rows, row)
	}
	return columns, rows
}

// renderMap draws the pairs a RESP3 reply arrives as — `CONFIG GET`, `XINFO STREAM`, a hash read
// whole — in field order, because a Go map has none and the same reply drawn twice would otherwise
// be drawn two ways.
func renderMap(pairs map[any]any) ([]driver.Column, [][]*string) {
	fields := make([]string, 0, len(pairs))
	held := make(map[string]any, len(pairs))
	for name, value := range pairs {
		field := fmt.Sprint(name)
		fields = append(fields, field)
		held[field] = value
	}
	sort.Strings(fields)

	rows := make([][]*string, 0, len(fields))
	for _, field := range fields {
		rows = append(rows, []*string{cell(field), replyCell(held[field])})
	}
	return []driver.Column{replyColumn(fieldLabel), replyColumn(valueLabel)}, rows
}

// scalar is one value of a reply as text. A nested list inside a list of lists is written as JSON
// rather than as Go's own spelling of a slice, so a cell the reader opens holds something they can
// read — and something the value viewer can lay out.
func replyCell(value any) *string {
	switch typed := value.(type) {
	case nil:
		return nil
	case string:
		return cell(typed)
	case int64:
		return cell(strconv.FormatInt(typed, 10))
	case float64:
		return cell(strconv.FormatFloat(typed, 'f', -1, 64))
	case bool:
		return cell(strconv.FormatBool(typed))
	case []byte:
		return cell(string(typed))
	case []any:
		texts := make([]string, 0, len(typed))
		for _, one := range typed {
			texts = append(texts, textOf(replyCell(one)))
		}
		return cell(jsonArray(texts))
	case map[any]any:
		pairs := make([][2]string, 0, len(typed))
		for name, one := range typed {
			pairs = append(pairs, [2]string{fmt.Sprint(name), textOf(replyCell(one))})
		}
		sort.Slice(pairs, func(a, b int) bool { return pairs[a][0] < pairs[b][0] })
		return cell(jsonPairs(pairs))
	default:
		return cell(fmt.Sprint(typed))
	}
}

func textOf(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// column is a result column of a reply. Nothing here carries a type: a Redis reply is bytes, a
// number or a list of either, and naming a type for it would be naming one the server never did.
func replyColumn(name string) driver.Column {
	return driver.Column{Name: name, Type: "reply", FullType: "reply", Nullable: true}
}
