package mongo

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func explained(t *testing.T, document bson.D) bson.Raw {
	t.Helper()

	raw, err := bson.Marshal(document)
	if err != nil {
		t.Fatalf("marshalling the explain document: %v", err)
	}
	return raw
}

// A find explained: one winning plan, its stages feeding one another downward.
func findPlan() bson.D {
	return bson.D{
		{Key: "queryPlanner", Value: bson.D{
			{Key: "namespace", Value: "shop.orders"},
			{Key: "winningPlan", Value: bson.D{
				{Key: "stage", Value: "FETCH"},
				{Key: "inputStage", Value: bson.D{
					{Key: "stage", Value: "IXSCAN"},
					{Key: "indexName", Value: "customer_id_1"},
					{Key: "direction", Value: "forward"},
					{Key: "keyPattern", Value: bson.D{{Key: "customer_id", Value: 1}}},
				}},
			}},
			{Key: "rejectedPlans", Value: bson.A{bson.D{{Key: "stage", Value: "COLLSCAN"}}}},
		}},
	}
}

func TestReadPlanFlattensAWinningPlanTopDown(t *testing.T) {
	plan := readPlan(explained(t, findPlan()))
	if plan == nil {
		t.Fatal("readPlan returned no plan")
	}

	if len(plan.Nodes) != 2 {
		t.Fatalf("read %d stages, want 2: %+v", len(plan.Nodes), plan.Nodes)
	}
	if plan.Nodes[0].Type != "FETCH" || plan.Nodes[0].Depth != 0 {
		t.Errorf("first stage = %q at depth %d, want FETCH at 0",
			plan.Nodes[0].Type, plan.Nodes[0].Depth)
	}
	if plan.Nodes[1].Type != "IXSCAN" || plan.Nodes[1].Depth != 1 {
		t.Errorf("second stage = %q at depth %d, want IXSCAN at 1",
			plan.Nodes[1].Type, plan.Nodes[1].Depth)
	}
}

// A document store prices nothing, and the plan says so rather than carrying a column of zeroes.
func TestReadPlanIsNotCosted(t *testing.T) {
	plan := readPlan(explained(t, findPlan()))
	if plan == nil {
		t.Fatal("readPlan returned no plan")
	}

	if plan.Costed {
		t.Error("plan claims MongoDB prices a stage")
	}
	for _, node := range plan.Nodes {
		if node.Cost != 0 || node.SelfCost != 0 || node.Rows != 0 {
			t.Errorf("stage %q was given numbers the server never gave: %+v", node.Type, node)
		}
	}
}

func TestReadPlanDetailNamesTheIndexAStageRead(t *testing.T) {
	plan := readPlan(explained(t, findPlan()))
	if plan == nil {
		t.Fatal("readPlan returned no plan")
	}

	want := `indexName: customer_id_1  direction: forward  keyPattern: {"customer_id":1}`
	if got := plan.Nodes[1].Detail; got != want {
		t.Errorf("IXSCAN detail = %q, want %q", got, want)
	}
}

func TestReadPlanSummaryReportsTheWinnerAndWhatLost(t *testing.T) {
	plan := readPlan(explained(t, findPlan()))
	if plan == nil {
		t.Fatal("readPlan returned no plan")
	}

	want := map[string]string{
		"Winning stage":  "FETCH",
		"Stages":         "2",
		"Namespace":      "shop.orders",
		"Rejected plans": "1",
	}
	for _, fact := range plan.Summary {
		if expected, ok := want[fact.Label]; ok && fact.Value != expected {
			t.Errorf("%s = %q, want %q", fact.Label, fact.Value, expected)
		}
		delete(want, fact.Label)
	}
	for label := range want {
		t.Errorf("summary said nothing about %s", label)
	}
}

// A server on the slot-based engine reports the same plan twice, and it is the stages under
// `queryPlan` that are the ones a reader reads.
func TestReadPlanReadsTheStagesOfASlotBasedPlan(t *testing.T) {
	plan := readPlan(explained(t, bson.D{
		{Key: "explainVersion", Value: "2"},
		{Key: "queryPlanner", Value: bson.D{
			{Key: "winningPlan", Value: bson.D{
				{Key: "queryPlan", Value: bson.D{
					{Key: "stage", Value: "FETCH"},
					{Key: "planNodeId", Value: 1},
					{Key: "inputStage", Value: bson.D{
						{Key: "stage", Value: "IXSCAN"},
						{Key: "indexName", Value: "customer_id_1"},
					}},
				}},
				{Key: "slotBasedPlan", Value: bson.D{{Key: "slots", Value: "$$RESULT=s11"}}},
			}},
		}},
	}))
	if plan == nil {
		t.Fatal("readPlan returned no plan")
	}

	if len(plan.Nodes) != 2 {
		t.Fatalf("read %d stages, want 2: %+v", len(plan.Nodes), plan.Nodes)
	}
	if plan.Nodes[0].Type != "FETCH" || plan.Nodes[1].Type != "IXSCAN" {
		t.Errorf("read %q then %q, want FETCH then IXSCAN", plan.Nodes[0].Type, plan.Nodes[1].Type)
	}
}

// An aggregate is explained as its pipeline, and the `$cursor` feeding it holds a plan of its own.
func TestReadPlanFlattensAPipelineAndThePlanFeedingIt(t *testing.T) {
	plan := readPlan(explained(t, bson.D{
		{Key: "stages", Value: bson.A{
			bson.D{{Key: "$cursor", Value: bson.D{
				{Key: "queryPlanner", Value: bson.D{
					{Key: "winningPlan", Value: bson.D{{Key: "stage", Value: "COLLSCAN"}}},
				}},
			}}},
			bson.D{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$customer_id"}}}},
		}},
	}))
	if plan == nil {
		t.Fatal("readPlan returned no plan")
	}

	want := []struct {
		Type  string
		Depth int
	}{
		{"$cursor", 0},
		{"COLLSCAN", 1},
		{"$group", 0},
	}
	if len(plan.Nodes) != len(want) {
		t.Fatalf("read %d stages, want %d: %+v", len(plan.Nodes), len(want), plan.Nodes)
	}
	for i, one := range want {
		if plan.Nodes[i].Type != one.Type || plan.Nodes[i].Depth != one.Depth {
			t.Errorf("stage %d = %q at depth %d, want %q at depth %d",
				i, plan.Nodes[i].Type, plan.Nodes[i].Depth, one.Type, one.Depth)
		}
	}
}

// A command whose answer holds no stages this side recognises invents none — and still hands back
// the document, because the JSON tab is where a shape nobody anticipated is read.
func TestReadPlanInventsNoStagesForAnAnswerHoldingNone(t *testing.T) {
	plan := readPlan(explained(t, bson.D{{Key: "ok", Value: 1}}))
	if plan == nil {
		t.Fatal("readPlan dropped a document it could read")
	}

	if len(plan.Nodes) != 0 {
		t.Errorf("read stages out of a document that holds none: %+v", plan.Nodes)
	}
	if plan.JSON == "" {
		t.Error("the document itself was dropped")
	}
}
