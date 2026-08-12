package sqlserver

import (
	"context"
	"strconv"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// explainPlan asks SQL Server what the statement is going to do. There is no EXPLAIN here: the
// server is put into a *mode* in which the next statement is compiled and described instead of run,
// and then taken out of it again.
//
// That mode is per connection, which is why this takes the pinned one rather than the pool: a
// setting left on somebody else's connection would silently stop their statements running. It is
// turned off in a defer for the same reason.
//
// The estimated form is the only one asked for. `SHOWPLAN` describes without executing, which is
// what Explain has to mean — the statement itself is run exactly once, by the caller.
func explainPlan(ctx context.Context, conn sqlkit.Queryer, statement string) *driver.Plan {
	if _, err := conn.ExecContext(ctx, "SET SHOWPLAN_ALL ON"); err != nil {
		return nil
	}
	defer conn.ExecContext(ctx, "SET SHOWPLAN_ALL OFF")

	explained, err := sqlkit.Read(ctx, conn, statement, 0, nil)
	if err != nil {
		return nil
	}

	plan := sqlkit.EmptyPlan()
	plan.Columns, plan.Rows = explained.Columns, explained.Rows
	plan.Nodes = nodesOf(explained.Columns, explained.Rows)
	plan.Summary = summaryOf(plan.Nodes)
	plan.Costed = true
	return plan
}

// showplanColumns are the ones a step is read out of, out of the two dozen SHOWPLAN_ALL answers
// with. They are found by name rather than by position, because the position is a promise the
// server never made.
const (
	columnStatement = "StmtText"
	columnOperation = "PhysicalOp"
	columnArgument  = "Argument"
	columnObject    = "TableName"
	columnRows      = "EstimateRows"
	columnTotalCost = "TotalSubtreeCost"
	columnIOCost    = "EstimateIO"
	columnCPUCost   = "EstimateCPU"
	columnParent    = "Parent"
	columnNodeID    = "NodeId"
)

// nodesOf reads the rows into the tree they describe. SHOWPLAN_ALL says which step is whose parent,
// and the rows arrive in the order the tree reads, so one pass settles the depth of each.
//
// The first row is the statement itself rather than a step — it has no physical operation — and it
// is what carries the whole plan's cost, which is why the summary reads it and the tree does not.
func nodesOf(columns []driver.Column, rows [][]*string) []driver.PlanNode {
	at := index(columns)
	depths := map[string]int{}

	nodes := make([]driver.PlanNode, 0, len(rows))
	for _, row := range rows {
		read := func(name string) string { return cell(row, at, name) }

		operation := read(columnOperation)
		if operation == "" {
			// The statement's own row. It is the plan's total rather than a step in it.
			continue
		}

		parent := read(columnParent)
		depth := depths[parent]
		if parent == "" || parent == "0" {
			depth = 0
		}
		depths[read(columnNodeID)] = depth + 1

		nodes = append(nodes, driver.PlanNode{
			Type:     operation,
			Detail:   detailOf(read),
			Depth:    depth,
			Rows:     number(read(columnRows)),
			Cost:     number(read(columnTotalCost)),
			SelfCost: number(read(columnIOCost)) + number(read(columnCPUCost)),
		})
	}
	return nodes
}

// detailOf is what the step works on, in one line: the object it reads and the argument that says
// how. The argument is where the index and the predicate are, and it is the thing a reader
// explaining a slow statement is looking for.
func detailOf(read func(string) string) string {
	parts := []string{}
	if object := read(columnObject); object != "" {
		parts = append(parts, "on "+object)
	}
	if argument := read(columnArgument); argument != "" {
		parts = append(parts, argument)
	}
	return sqlkit.JoinDetail(parts)
}

// summaryOf counts what the plan holds and what it costs. The cost of the whole is the root step's
// subtree cost, which is what SQL Server means by a query's cost and the number every other tool
// shows beside one.
func summaryOf(nodes []driver.PlanNode) []driver.PlanFact {
	total, rows, scans := 0.0, 0.0, 0
	for _, node := range nodes {
		if node.Depth == 0 {
			total += node.Cost
			rows += node.Rows
		}
		if strings.Contains(node.Type, "Scan") && !strings.Contains(node.Type, "Index Seek") {
			scans++
		}
	}

	return []driver.PlanFact{
		{Label: "Query cost", Value: sqlkit.Cost(total)},
		{Label: "Estimated rows", Value: sqlkit.Rows(rows)},
		{Label: "Steps", Value: sqlkit.Rows(float64(len(nodes)))},
		{Label: "Scans", Value: sqlkit.Rows(float64(scans))},
	}
}

func index(columns []driver.Column) map[string]int {
	at := make(map[string]int, len(columns))
	for i, column := range columns {
		at[column.Name] = i
	}
	return at
}

func cell(row []*string, at map[string]int, name string) string {
	position, ok := at[name]
	if !ok || position >= len(row) || row[position] == nil {
		return ""
	}
	return strings.TrimSpace(*row[position])
}

func number(value string) float64 {
	parsed, _ := strconv.ParseFloat(value, 64)
	return parsed
}
