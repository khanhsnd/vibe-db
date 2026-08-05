package mysql

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// steps are the keys MySQL nests one step of a plan under, and what to call each. The walk goes by
// this list rather than by every key it meets because most of an object's keys *describe* the step
// holding them — `cost_info`, `used_columns`, `select_id` — and only these carry another step.
//
// A key holding a **list** is a step in its own right whose entries sit under it: `nested_loop` is
// the join, and the tables it holds are what is joined.
var steps = []struct {
	Key  string
	Name string
}{
	{"query_block", "Query block"},
	{"union_result", "Union"},
	{"query_specifications", "Query specification"},
	{"ordering_operation", "Ordering"},
	{"grouping_operation", "Grouping"},
	{"duplicates_removal", "Duplicates removal"},
	{"buffer_result", "Buffer result"},
	{"windowing", "Windowing"},
	{"nested_loop", "Nested loop"},
	// A table is named after the way it is read — `ALL`, `ref`, `eq_ref` — that being the one thing
	// a reader is looking for when they explain a statement at all.
	{"table", ""},
	{"materialized_from_subquery", "Materialized subquery"},
	{"insert_from", "Insert from"},
	{"attached_subqueries", "Attached subquery"},
	{"select_list_subqueries", "Select list subquery"},
	{"optimized_away_subqueries", "Optimized-away subquery"},
	{"having_subqueries", "Having subquery"},
	{"group_by_subqueries", "Group by subquery"},
	{"order_by_subqueries", "Order by subquery"},
	{"update_value_subqueries", "Update value subquery"},
}

// flags are what a step says about *how* it does its work, and they are worth a reader's attention
// more than anything else in the document: a filesort and a temporary table are the two answers
// somebody explaining a slow statement is usually looking for.
var flags = []string{
	"using_filesort",
	"using_temporary_table",
	"using_index",
	"using_index_for_group_by",
	"using_join_buffer",
	"message",
}

// explainPlan asks MySQL what the statement is going to do, in both the forms a reader reads: the
// `EXPLAIN` they would have typed themselves, and the JSON form that carries a cost.
//
// Either can fail on its own and the other is still worth showing; nothing at all is no plan rather
// than a failure, the statement itself having run and its rows being what the reader asked for.
// Neither form is ever the ANALYZE one — the statement is run exactly once, by the caller.
func explainPlan(ctx context.Context, q sqlkit.Queryer, statement string) *driver.Plan {
	plan := readJSONPlan(ctx, q, statement)

	explained, err := sqlkit.Read(ctx, q, "EXPLAIN "+statement, 0, nil)
	if err != nil {
		return plan
	}
	if plan == nil {
		plan = sqlkit.EmptyPlan()
	}

	plan.Columns, plan.Rows = explained.Columns, explained.Rows
	return plan
}

func readJSONPlan(ctx context.Context, q sqlkit.Queryer, statement string) *driver.Plan {
	document, err := sqlkit.JSONPlan(ctx, q, "EXPLAIN FORMAT=JSON "+statement)
	if err != nil {
		return nil
	}
	return readPlan(document)
}

func readPlan(document string) *driver.Plan {
	explained := map[string]any{}
	if err := json.Unmarshal([]byte(document), &explained); err != nil {
		return nil
	}
	block, ok := explained["query_block"].(map[string]any)
	if !ok {
		return nil
	}

	read := &walk{nodes: []driver.PlanNode{}}
	read.step("Query block", block, 0)

	return &driver.Plan{
		Nodes:   read.nodes,
		Summary: read.summary(block),
		JSON:    document,
		Costed:  true,
	}
}

// walk is the reading in progress: the steps flattened so far, and the two things the summary counts
// while they go by. The count is kept here because only the walk knows which steps were *tables* —
// nothing on a flattened step says so, and nothing should, since a step is drawn and not classified.
type walk struct {
	nodes  []driver.PlanNode
	tables int
	// rows is what the last table handed upward, which for a join is what the whole read produces.
	rows float64
}

func (w *walk) step(name string, step map[string]any, depth int) {
	table := name == ""
	if table {
		name = sqlkit.PlanText(step["access_type"])
		w.tables++
		w.rows = rowsOf(step)
	}

	w.nodes = append(w.nodes, driver.PlanNode{
		Type:     name,
		Detail:   stepDetail(step),
		Depth:    depth,
		Rows:     rowsOf(step),
		Cost:     costOf(step),
		SelfCost: ownCost(step),
	})
	w.descend(step, depth+1)
}

// descend follows whichever steps this one holds, in the order the list above gives rather than the
// order the document happens to be written in, so two plans of one shape read the same way.
func (w *walk) descend(step map[string]any, depth int) {
	for _, nested := range steps {
		switch held := step[nested.Key].(type) {
		case map[string]any:
			w.step(nested.Name, held, depth)
		case []any:
			w.nodes = append(w.nodes, driver.PlanNode{Type: nested.Name, Depth: depth})
			for _, one := range held {
				if entry, ok := one.(map[string]any); ok {
					w.descend(entry, depth+1)
				}
			}
		}
	}
}

func (w *walk) summary(block map[string]any) []driver.PlanFact {
	return []driver.PlanFact{
		{Label: "Query cost", Value: sqlkit.Cost(costOf(block))},
		{Label: "Estimated rows", Value: sqlkit.Rows(w.rows)},
		{Label: "Steps", Value: sqlkit.Rows(float64(len(w.nodes)))},
		{Label: "Tables read", Value: sqlkit.Rows(float64(w.tables))},
	}
}

func stepDetail(step map[string]any) string {
	parts := []string{}
	if name := sqlkit.PlanText(step["table_name"]); name != "" {
		parts = append(parts, "on "+name)
	}
	if key := sqlkit.PlanText(step["key"]); key != "" {
		parts = append(parts, "using "+key)
	}
	if filtered := sqlkit.PlanText(step["filtered"]); filtered != "" {
		parts = append(parts, "filtered: "+filtered+"%")
	}

	for _, flag := range flags {
		switch value := step[flag].(type) {
		case bool:
			if value {
				parts = append(parts, strings.ReplaceAll(flag, "_", " "))
			}
		case string:
			parts = append(parts, strings.ReplaceAll(flag, "_", " ")+": "+value)
		}
	}

	if condition := sqlkit.PlanText(step["attached_condition"]); condition != "" {
		parts = append(parts, condition)
	}
	return sqlkit.JoinDetail(parts)
}

// costOf is what a step costs including everything it reads: a block's own `query_cost`, and for a
// table the prefix cost, which is that table and everything joined ahead of it.
func costOf(step map[string]any) float64 {
	if info, ok := step["cost_info"].(map[string]any); ok {
		if cost, held := info["query_cost"]; held {
			return sqlkit.PlanNumber(cost)
		}
		return sqlkit.PlanNumber(info["prefix_cost"])
	}
	return 0
}

// ownCost is what the step costs on its own, and **only a table has one**. MySQL prices reading a
// table and evaluating its rows; it does not price the sort, the grouping or the join that sit above
// them, so those are left at zero rather than given a number worked out here. What that buys is a
// statistics tab whose costs add up to the query's, which is the only thing a share of the cost can
// honestly be a share of.
func ownCost(step map[string]any) float64 {
	info, ok := step["cost_info"].(map[string]any)
	if !ok {
		return 0
	}
	return sqlkit.PlanNumber(info["read_cost"]) + sqlkit.PlanNumber(info["eval_cost"])
}

// rowsOf is how many rows the step hands upward. A table reports both what it scans and what the
// join then produces, and it is the second that says what the step gives the one above it.
func rowsOf(step map[string]any) float64 {
	if rows, ok := step["rows_produced_per_join"]; ok {
		return sqlkit.PlanNumber(rows)
	}
	return sqlkit.PlanNumber(step["rows_examined_per_scan"])
}
