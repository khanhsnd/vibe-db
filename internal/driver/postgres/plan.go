package postgres

import (
	"context"
	"encoding/json"
	"math"

	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// conditions are the fields a step names its work in, in the order they read. Each is written out
// under the server's own label, because `Index Cond` and `Filter` are two different things and a
// reader deciding whether an index is doing its job is deciding between exactly those two.
var conditions = []string{
	"Index Cond",
	"Recheck Cond",
	"Hash Cond",
	"Merge Cond",
	"Join Filter",
	"Filter",
	"Sort Key",
	"Group Key",
	"Join Type",
}

// explainPlan asks PostgreSQL what the statement is going to do, in both the forms a reader reads:
// the `EXPLAIN` they would have typed themselves, and the JSON form that carries a number per step.
//
// Either can fail on its own — a statement PostgreSQL will not explain at all, a document that will
// not parse — and the other is still worth showing. Nothing at all is no plan rather than a failure:
// the statement itself has run, and its rows are what the reader asked for.
//
// Neither form is ever the ANALYZE one. The statement is run exactly once, by the caller.
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
	document, err := sqlkit.JSONPlan(ctx, q, "EXPLAIN (FORMAT JSON) "+statement)
	if err != nil {
		return nil
	}
	return readPlan(document)
}

// readPlan reads PostgreSQL's own plan document: a list of one entry per statement, each holding
// the root step under `Plan` and whatever the server measured about the whole beside it.
func readPlan(document string) *driver.Plan {
	explained := []map[string]any{}
	if err := json.Unmarshal([]byte(document), &explained); err != nil || len(explained) == 0 {
		return nil
	}
	root, ok := explained[0]["Plan"].(map[string]any)
	if !ok {
		return nil
	}

	plan := &driver.Plan{Nodes: []driver.PlanNode{}, JSON: document, Costed: true}
	appendStep(plan, root, 0)
	plan.Summary = planSummary(explained[0], root, len(plan.Nodes))
	return plan
}

// appendStep flattens one step and everything under it. A step's own cost is what it costs beyond
// what it reads — the server's `Total Cost` includes its children — which is what makes the
// statistics add up to the plan rather than to several times it.
func appendStep(plan *driver.Plan, step map[string]any, depth int) {
	children, _ := step["Plans"].([]any)

	cost := sqlkit.PlanNumber(step["Total Cost"])
	own := cost
	for _, child := range children {
		if one, ok := child.(map[string]any); ok {
			own -= sqlkit.PlanNumber(one["Total Cost"])
		}
	}

	plan.Nodes = append(plan.Nodes, driver.PlanNode{
		Type:     sqlkit.PlanText(step["Node Type"]),
		Detail:   stepDetail(step),
		Depth:    depth,
		Rows:     sqlkit.PlanNumber(step["Plan Rows"]),
		Cost:     cost,
		SelfCost: math.Max(own, 0),
	})

	for _, child := range children {
		if one, ok := child.(map[string]any); ok {
			appendStep(plan, one, depth+1)
		}
	}
}

func stepDetail(step map[string]any) string {
	parts := []string{}
	if name := sqlkit.PlanText(step["Relation Name"]); name != "" {
		parts = append(parts, "on "+name)
	}
	if index := sqlkit.PlanText(step["Index Name"]); index != "" {
		parts = append(parts, "using "+index)
	}
	if name := sqlkit.PlanText(step["Subplan Name"]); name != "" {
		parts = append(parts, name)
	}
	for _, key := range conditions {
		if text := sqlkit.PlanText(step[key]); text != "" {
			parts = append(parts, key+": "+text)
		}
	}
	return sqlkit.JoinDetail(parts)
}

// planSummary is what the plan says about itself. The two times are reported only when the server
// reported them, since an estimate has no time to give.
func planSummary(explained, root map[string]any, steps int) []driver.PlanFact {
	facts := []driver.PlanFact{
		{Label: "Total cost", Value: sqlkit.Cost(sqlkit.PlanNumber(root["Total Cost"]))},
		{Label: "Startup cost", Value: sqlkit.Cost(sqlkit.PlanNumber(root["Startup Cost"]))},
		{Label: "Estimated rows", Value: sqlkit.Rows(sqlkit.PlanNumber(root["Plan Rows"]))},
		{Label: "Steps", Value: sqlkit.Rows(float64(steps))},
	}

	for _, timed := range []string{"Planning Time", "Execution Time"} {
		if reported, ok := explained[timed]; ok {
			facts = append(facts, driver.PlanFact{
				Label: timed,
				Value: sqlkit.Cost(sqlkit.PlanNumber(reported)) + " ms",
			})
		}
	}
	return facts
}
