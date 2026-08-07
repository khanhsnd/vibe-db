package oracle

import (
	"context"
	"database/sql"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// explainPlan asks Oracle what the statement is going to do. Oracle's EXPLAIN answers with nothing:
// it *writes* the plan into a table — `plan_table`, which every instance ships with a public
// synonym for — and reading it back is a second statement.
//
// That is why this needs the pinned connection rather than the pool. The rows are written in the
// session's own transaction and nothing has committed them, so a read from another connection in
// the pool would find the plan of whatever was explained last, or nothing at all.
//
// It is never the `ANALYZE`-like form: EXPLAIN PLAN compiles and does not run, which is what
// Explain has to mean — the statement itself is run exactly once, by the caller.
func explainPlan(ctx context.Context, conn *sql.Conn, statement string) *driver.Plan {
	if _, err := conn.ExecContext(ctx, "EXPLAIN PLAN FOR "+statement); err != nil {
		return nil
	}

	plan := sqlkit.EmptyPlan()
	plan.Nodes, plan.Costed = nodesOf(ctx, conn), true
	plan.Summary = summaryOf(plan.Nodes)

	// The reader's own form as well as the read one: DBMS_XPLAN is the answer somebody typing
	// EXPLAIN PLAN by hand would have looked at, and it says things the table does not — which
	// predicates were applied where, and why a plan was chosen.
	if displayed, err := sqlkit.Read(ctx, conn, display, 0, nil); err == nil {
		plan.Columns, plan.Rows = displayed.Columns, displayed.Rows
	}
	return plan
}

// display is Oracle's own rendering of the plan it just wrote. `DISPLAY` with no arguments reads
// the most recent plan of this session, which is the one just explained.
const display = `SELECT plan_table_output AS "Plan" FROM TABLE(DBMS_XPLAN.DISPLAY())`

// steps reads the plan back out of the table Oracle wrote it into. `plan_id` is what tells one
// explain from the next, and the newest is the one just written.
//
// `depth` is Oracle's own column and says how deep the step sits, so unlike every other engine here
// the tree needs no walking: the rows arrive in the order the tree reads and already know where
// they belong.
const steps = `
	SELECT NVL(depth, 0),
	       operation || NVL2(options, ' ' || options, ''),
	       NVL2(object_name, object_owner || '.' || object_name, ''),
	       NVL(cardinality, 0),
	       NVL(cost, 0),
	       NVL(access_predicates, ''),
	       NVL(filter_predicates, '')
	FROM plan_table
	WHERE plan_id = (SELECT MAX(plan_id) FROM plan_table)
	ORDER BY id`

func nodesOf(ctx context.Context, conn *sql.Conn) []driver.PlanNode {
	rows, err := conn.QueryContext(ctx, steps)
	if err != nil {
		return []driver.PlanNode{}
	}
	defer rows.Close()

	nodes := []driver.PlanNode{}
	for rows.Next() {
		var depth int
		var operation, object, access, filter string
		var cardinality, cost float64

		err := rows.Scan(&depth, &operation, &object, &cardinality, &cost, &access, &filter)
		if err != nil {
			return nodes
		}

		nodes = append(nodes, driver.PlanNode{
			Type:   operation,
			Detail: detailOf(object, access, filter),
			Depth:  depth,
			Rows:   cardinality,
			Cost:   cost,
			// Oracle prices a step *including* everything under it and reports no figure for the
			// step alone, so nothing is put here rather than a number worked out on this side. The
			// same rule MySQL's plan follows for a step that is not a table.
		})
	}
	return nodes
}

// detailOf is what the step works on, in one line: the object it reads and the predicates that say
// how. The access predicate is what an index was probed with and the filter is what was thrown away
// afterwards, which is the distinction a reader explaining a slow statement is looking for.
func detailOf(object, access, filter string) string {
	parts := []string{}
	if object != "" {
		parts = append(parts, "on "+strings.TrimPrefix(object, "."))
	}
	if access != "" {
		parts = append(parts, "access: "+access)
	}
	if filter != "" {
		parts = append(parts, "filter: "+filter)
	}
	return sqlkit.JoinDetail(parts)
}

// summaryOf counts what the plan holds and what it costs. The cost of the whole is the root step's,
// Oracle pricing a step including everything under it — so the first row of the plan is the price
// of all of it, and is the number every other tool shows beside a statement.
func summaryOf(nodes []driver.PlanNode) []driver.PlanFact {
	total, rows, scans := 0.0, 0.0, 0
	for _, node := range nodes {
		if node.Depth == 0 {
			total, rows = node.Cost, node.Rows
		}
		if strings.Contains(node.Type, "TABLE ACCESS FULL") {
			scans++
		}
	}

	return []driver.PlanFact{
		{Label: "Query cost", Value: sqlkit.Cost(total)},
		{Label: "Estimated rows", Value: sqlkit.Rows(rows)},
		{Label: "Steps", Value: sqlkit.Rows(float64(len(nodes)))},
		{Label: "Full scans", Value: sqlkit.Rows(float64(scans))},
	}
}
