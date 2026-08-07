package sqlite

import (
	"context"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// explainPlan asks SQLite what the statement is going to do. There is one form and it is the one a
// reader would have typed: `EXPLAIN QUERY PLAN` answers with a row per step, each naming its parent,
// which is a tree written down as a table.
//
// It is never the bare `EXPLAIN`, which disassembles the statement into the virtual machine's own
// opcodes — a hundred rows of `OpenRead` and `Next` that say nothing about which index was chosen.
//
// Nothing at all is no plan rather than a failure: the statement itself has run and its rows are
// what the reader asked for.
func explainPlan(ctx context.Context, q sqlkit.Queryer, statement string) *driver.Plan {
	explained, err := sqlkit.Read(ctx, q, "EXPLAIN QUERY PLAN "+statement, 0, nil)
	if err != nil {
		return nil
	}

	plan := sqlkit.EmptyPlan()
	plan.Columns, plan.Rows = explained.Columns, explained.Rows
	plan.Nodes = nodesOf(explained.Rows)
	plan.Summary = summaryOf(plan.Nodes)

	// SQLite puts no number on a step. It picks a plan by rules and by whatever ANALYZE last
	// wrote, and reports neither a cost nor a row estimate — so the plan is drawn without those
	// columns rather than with columns full of zeroes.
	plan.Costed = false
	return plan
}

// nodesOf reads the flat rows back into the tree they describe. Each row is `id`, `parent`,
// `notused`, `detail`; the rows arrive in the order the tree reads, so a step's depth is one more
// than its parent's and a single pass is enough.
//
// The step's *type* is the first word of its detail — `SCAN`, `SEARCH`, `USE`, `CORRELATED` — which
// is SQLite's own name for what it is about to do and the word a reader is looking for. The rest is
// the detail, exactly as the server wrote it.
func nodesOf(rows [][]*string) []driver.PlanNode {
	depths := map[string]int{}

	nodes := make([]driver.PlanNode, 0, len(rows))
	for _, row := range rows {
		if len(row) < 4 {
			continue
		}

		id, parent, detail := text(row[0]), text(row[1]), text(row[3])
		depth := depths[parent]
		if parent == "" || parent == "0" {
			depth = 0
		}
		depths[id] = depth + 1

		kind, rest, found := strings.Cut(detail, " ")
		if !found {
			kind, rest = detail, ""
		}
		nodes = append(nodes, driver.PlanNode{Type: kind, Detail: rest, Depth: depth})
	}
	return nodes
}

// summaryOf counts what the plan holds. There is no cost to report, so what is worth saying is how
// much work there is and how much of it is a scan — a SEARCH uses an index and a SCAN reads the
// whole table, which is the one distinction `EXPLAIN QUERY PLAN` exists to draw.
func summaryOf(nodes []driver.PlanNode) []driver.PlanFact {
	scans := 0
	for _, node := range nodes {
		if node.Type == "SCAN" {
			scans++
		}
	}

	return []driver.PlanFact{
		{Label: "Steps", Value: sqlkit.Rows(float64(len(nodes)))},
		{Label: "Full scans", Value: sqlkit.Rows(float64(scans))},
	}
}

func text(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
