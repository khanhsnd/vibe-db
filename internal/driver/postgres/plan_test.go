package postgres

import "testing"

// A nested loop over an index scan and a sequential scan, as `EXPLAIN (FORMAT JSON)` writes one.
const planDocument = `[
  {
    "Plan": {
      "Node Type": "Nested Loop",
      "Join Type": "Inner",
      "Startup Cost": 0.29,
      "Total Cost": 16.58,
      "Plan Rows": 4,
      "Plans": [
        {
          "Node Type": "Seq Scan",
          "Relation Name": "customers",
          "Total Cost": 1.04,
          "Plan Rows": 4,
          "Filter": "(country = 'VN'::text)"
        },
        {
          "Node Type": "Index Scan",
          "Relation Name": "orders",
          "Index Name": "orders_customer_id_idx",
          "Total Cost": 3.85,
          "Plan Rows": 1,
          "Index Cond": "(customer_id = customers.id)"
        }
      ]
    }
  }
]`

func TestReadPlanFlattensTheTreeInReadingOrder(t *testing.T) {
	plan := readPlan(planDocument)
	if plan == nil {
		t.Fatal("readPlan returned no plan")
	}

	want := []struct {
		Type  string
		Depth int
	}{
		{"Nested Loop", 0},
		{"Seq Scan", 1},
		{"Index Scan", 1},
	}
	if len(plan.Nodes) != len(want) {
		t.Fatalf("read %d steps, want %d", len(plan.Nodes), len(want))
	}
	for i, one := range want {
		if plan.Nodes[i].Type != one.Type || plan.Nodes[i].Depth != one.Depth {
			t.Errorf("step %d = %q at depth %d, want %q at depth %d",
				i, plan.Nodes[i].Type, plan.Nodes[i].Depth, one.Type, one.Depth)
		}
	}
}

// A step's own cost is what it costs beyond what it reads, so the shares add up to the plan rather
// than to several times it.
func TestReadPlanPricesAStepWithoutItsChildren(t *testing.T) {
	plan := readPlan(planDocument)
	if plan == nil {
		t.Fatal("readPlan returned no plan")
	}

	if got := plan.Nodes[0].SelfCost; got < 11.68 || got > 11.70 {
		t.Errorf("nested loop's own cost = %v, want 16.58 less its children's 4.89", got)
	}
	if got := plan.Nodes[1].SelfCost; got != 1.04 {
		t.Errorf("seq scan's own cost = %v, want its whole 1.04", got)
	}
}

func TestReadPlanDetailNamesWhatAStepReadsAndHow(t *testing.T) {
	plan := readPlan(planDocument)
	if plan == nil {
		t.Fatal("readPlan returned no plan")
	}

	tests := []struct {
		at   int
		want string
	}{
		{0, "Join Type: Inner"},
		{1, "on customers  Filter: (country = 'VN'::text)"},
		{2, "on orders  using orders_customer_id_idx  Index Cond: (customer_id = customers.id)"},
	}
	for _, tt := range tests {
		if got := plan.Nodes[tt.at].Detail; got != tt.want {
			t.Errorf("step %d detail = %q, want %q", tt.at, got, tt.want)
		}
	}
}

func TestReadPlanSummaryReportsTheRootAndTheStepCount(t *testing.T) {
	plan := readPlan(planDocument)
	if plan == nil {
		t.Fatal("readPlan returned no plan")
	}

	want := map[string]string{
		"Total cost":     "16.58",
		"Startup cost":   "0.29",
		"Estimated rows": "4",
		"Steps":          "3",
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

// A time is reported only when the server reported one, an estimate having no time to give.
func TestReadPlanReportsATimeOnlyWhenTheServerDid(t *testing.T) {
	plan := readPlan(planDocument)
	if plan == nil {
		t.Fatal("readPlan returned no plan")
	}
	for _, fact := range plan.Summary {
		if fact.Label == "Planning Time" || fact.Label == "Execution Time" {
			t.Errorf("summary invented a %s of %q", fact.Label, fact.Value)
		}
	}
}

func TestReadPlanRefusesADocumentThatIsNotOne(t *testing.T) {
	for _, document := range []string{"", "not json", "{}", "[]", `[{"Plan": 3}]`} {
		if plan := readPlan(document); plan != nil {
			t.Errorf("readPlan(%q) read a plan out of it", document)
		}
	}
}
