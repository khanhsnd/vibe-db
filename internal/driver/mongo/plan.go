package mongo

import (
	"encoding/json"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// worked are the fields a stage names its work in. `indexName` and `direction` are the two a reader
// explaining a read is looking for: whether an index was chosen, and which.
var worked = []string{
	"indexName",
	"direction",
	"filter",
	"sortPattern",
	"transformBy",
	"limitAmount",
	"skipAmount",
}

// readPlan reads a plan out of what the server answered `explain` with. A document store prices
// nothing — there is no cost to divide up and no row estimate to compare against — so the plan is
// the *shape* of the read and nothing more, and `Costed` says so rather than the grid guessing from
// a column of zeroes.
//
// It is a plan read from the very document the Rows already carry, and not a second call: the shape
// is in the answer already, and a reader who wants the answer itself has the JSON.
func readPlan(raw bson.Raw) *driver.Plan {
	document, err := bson.MarshalExtJSONIndent(raw, false, false, "", "  ")
	if err != nil {
		return nil
	}

	explained := map[string]any{}
	if err := json.Unmarshal(document, &explained); err != nil {
		return nil
	}

	plan := sqlkit.EmptyPlan()
	plan.JSON = string(document)
	appendStages(plan, explained, 0)
	plan.Summary = planSummary(explained, plan.Nodes)
	return plan
}

// appendStages flattens whichever of the two shapes an explain came back in. A find answers with a
// query planner holding one winning plan; an aggregate answers with its pipeline, each stage of
// which may hold a query planner of its own — the `$cursor` that feeds it.
func appendStages(plan *driver.Plan, explained map[string]any, depth int) {
	if planner, ok := explained["queryPlanner"].(map[string]any); ok {
		if winning, ok := planner["winningPlan"].(map[string]any); ok {
			appendStage(plan, winningStages(winning), depth)
		}
		return
	}

	stages, ok := explained["stages"].([]any)
	if !ok {
		return
	}
	for _, one := range stages {
		stage, ok := one.(map[string]any)
		if !ok {
			continue
		}
		// A stage is one operator and whatever the server measured beside it, so the operator is the
		// key spelled as one. Everything else describes it.
		for name, value := range stage {
			held, ok := value.(map[string]any)
			if !ok || !strings.HasPrefix(name, "$") {
				continue
			}
			plan.Nodes = append(plan.Nodes, driver.PlanNode{Type: name, Depth: depth})
			appendStages(plan, held, depth+1)
		}
	}
}

// winningStages is the stage tree of a winning plan, which is not always the winning plan itself: a
// server using the slot-based engine reports its plan twice — once as the stages a reader knows
// (`queryPlan`) and once as the slots it compiled them into — and it is the first that is a plan a
// reader can read. A server on the older engine writes the stages directly, and there is no
// `queryPlan` to find.
func winningStages(winning map[string]any) map[string]any {
	if stages, ok := winning["queryPlan"].(map[string]any); ok {
		return stages
	}
	return winning
}

// appendStage flattens one stage of a winning plan and whatever feeds it. A stage reads from its
// `inputStage`, or from several — a merged sort, an index intersection — and the plan is drawn from
// the top down, which is the reverse of the order the work happens in and the order the server
// writes it in.
func appendStage(plan *driver.Plan, stage map[string]any, depth int) {
	plan.Nodes = append(plan.Nodes, driver.PlanNode{
		Type:   sqlkit.PlanText(stage["stage"]),
		Detail: stageDetail(stage),
		Depth:  depth,
	})

	if input, ok := stage["inputStage"].(map[string]any); ok {
		appendStage(plan, input, depth+1)
	}
	inputs, _ := stage["inputStages"].([]any)
	for _, one := range inputs {
		if input, ok := one.(map[string]any); ok {
			appendStage(plan, input, depth+1)
		}
	}
}

func stageDetail(stage map[string]any) string {
	parts := []string{}
	for _, key := range worked {
		if said := sqlkit.PlanText(stage[key]); said != "" {
			parts = append(parts, key+": "+said)
		}
	}
	// A key pattern and the bounds it was read over are documents rather than values, so they are
	// written back out as the notation they arrived in.
	for _, key := range []string{"keyPattern", "indexBounds"} {
		if held, ok := stage[key]; ok {
			if written, err := json.Marshal(held); err == nil {
				parts = append(parts, key+": "+string(written))
			}
		}
	}
	return sqlkit.JoinDetail(parts)
}

// planSummary is what the plan says about itself. A command whose answer held no stages at all — a
// shape this side has not met — still has a summary saying so, rather than none.
func planSummary(explained map[string]any, nodes []driver.PlanNode) []driver.PlanFact {
	facts := []driver.PlanFact{{Label: "Stages", Value: sqlkit.Rows(float64(len(nodes)))}}
	if len(nodes) > 0 {
		facts = append(facts, driver.PlanFact{Label: "Winning stage", Value: nodes[0].Type})
	}

	planner, ok := explained["queryPlanner"].(map[string]any)
	if !ok {
		return facts
	}
	if namespace := sqlkit.PlanText(planner["namespace"]); namespace != "" {
		facts = append(facts, driver.PlanFact{Label: "Namespace", Value: namespace})
	}
	if rejected, ok := planner["rejectedPlans"].([]any); ok {
		facts = append(facts, driver.PlanFact{
			Label: "Rejected plans",
			Value: sqlkit.Rows(float64(len(rejected))),
		})
	}
	return facts
}
