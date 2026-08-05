package sqlkit

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/driver"
)

// JSONPlan runs an EXPLAIN that answers with a JSON document and hands it back laid out. Both SQL
// engines have such a form and both answer with it as text in a single column; what the document
// *says* is entirely theirs to read.
//
// It is deliberately never the ANALYZE form. Explain has to change nothing, whatever the statement
// under it would have done, so what is asked for is an estimate and never a run.
func JSONPlan(ctx context.Context, q Queryer, statement string) (string, error) {
	rows, err := q.QueryContext(ctx, statement)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	document := bytes.Buffer{}
	for rows.Next() {
		part := ""
		if err := rows.Scan(&part); err != nil {
			return "", err
		}
		document.WriteString(part)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}

	return IndentJSON(document.String()), nil
}

// EmptyPlan is a plan holding nothing yet, for the caller that read one of a plan's forms and not
// the others. Its lists are empty rather than nil so that a plan crosses the wire as `[]` and the
// grid drawing it has nothing to guard against.
func EmptyPlan() *driver.Plan {
	return &driver.Plan{
		Rows:    [][]*string{},
		Nodes:   []driver.PlanNode{},
		Summary: []driver.PlanFact{},
	}
}

// IndentJSON lays a plan document out over lines, and answers with it untouched when it is not JSON
// at all: what the reader is being shown is what the server said, and a server that said something
// else is better read raw than not read.
func IndentJSON(document string) string {
	out := bytes.Buffer{}
	if json.Indent(&out, []byte(document), "", "  ") != nil {
		return document
	}
	return out.String()
}

// PlanNumber reads a number out of a plan document whichever way the engine wrote it: PostgreSQL
// writes a JSON number and MySQL a string holding one.
func PlanNumber(value any) float64 {
	switch value := value.(type) {
	case float64:
		return value
	case string:
		number, _ := strconv.ParseFloat(value, 64)
		return number
	default:
		return 0
	}
}

// PlanText reads a plan field as the one line a step's detail is built from. A list — a sort key, a
// group key — is joined, since it describes one step and not several.
func PlanText(value any) string {
	switch value := value.(type) {
	case string:
		return value
	case bool:
		return strconv.FormatBool(value)
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case []any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			if text := PlanText(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, ", ")
	default:
		return ""
	}
}

// JoinDetail is how the parts of a step's detail meet: two spaces, so a condition that holds one
// space of its own still reads as a single part.
func JoinDetail(parts []string) string {
	return strings.Join(parts, "  ")
}

// Cost is a cost as a plan says it: two decimals, the unit being the engine's own and meaningless
// beyond comparing one step with another.
func Cost(cost float64) string {
	return strconv.FormatFloat(cost, 'f', 2, 64)
}

// Rows is a row estimate as a plan says it. Whole rows: an engine that estimates 2.5 of them is
// estimating, and a decimal place would suggest it had counted.
func Rows(rows float64) string {
	return strconv.FormatInt(int64(rows), 10)
}
