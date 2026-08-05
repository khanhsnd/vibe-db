package mysql

import (
	"testing"

	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// A sorted join of two tables, as `EXPLAIN FORMAT=JSON` writes one: every number a string, and the
// steps nested under keys that name them rather than under one uniform list.
const planDocument = `{
  "query_block": {
    "select_id": 1,
    "cost_info": {"query_cost": "3.20"},
    "ordering_operation": {
      "using_filesort": true,
      "nested_loop": [
        {
          "table": {
            "table_name": "customers",
            "access_type": "ALL",
            "rows_examined_per_scan": 4,
            "rows_produced_per_join": 4,
            "filtered": "100.00",
            "cost_info": {"read_cost": "0.25", "eval_cost": "0.40", "prefix_cost": "0.65"},
            "attached_condition": "(` + "`c`.`country`" + ` = 'VN')"
          }
        },
        {
          "table": {
            "table_name": "orders",
            "access_type": "ref",
            "key": "orders_customer_id_idx",
            "rows_examined_per_scan": 1,
            "rows_produced_per_join": 6,
            "filtered": "100.00",
            "using_index": true,
            "cost_info": {"read_cost": "1.35", "eval_cost": "0.60", "prefix_cost": "2.60"}
          }
        }
      ]
    }
  }
}`

func TestReadPlanFlattensTheKeysAStepIsNestedUnder(t *testing.T) {
	plan := readPlan(planDocument)
	if plan == nil {
		t.Fatal("readPlan returned no plan")
	}

	want := []struct {
		Type  string
		Depth int
	}{
		{"Query block", 0},
		{"Ordering", 1},
		{"Nested loop", 2},
		{"ALL", 3},
		{"ref", 3},
	}
	if len(plan.Nodes) != len(want) {
		t.Fatalf("read %d steps, want %d: %+v", len(plan.Nodes), len(want), plan.Nodes)
	}
	for i, one := range want {
		if plan.Nodes[i].Type != one.Type || plan.Nodes[i].Depth != one.Depth {
			t.Errorf("step %d = %q at depth %d, want %q at depth %d",
				i, plan.Nodes[i].Type, plan.Nodes[i].Depth, one.Type, one.Depth)
		}
	}
}

// A table is priced by what it costs to read and to evaluate; nothing above it is priced at all, so
// the shares of the cost add up to the query's own.
func TestReadPlanPricesOnlyTheTables(t *testing.T) {
	plan := readPlan(planDocument)
	if plan == nil {
		t.Fatal("readPlan returned no plan")
	}

	// Compared as the reader is shown them: a cost read out of two strings and added is a float, and
	// 1.35 plus 0.60 is not exactly 1.95 in any of them.
	tests := []struct {
		at   int
		cost string
		own  string
	}{
		{0, "3.20", "0.00"}, // the query block, priced whole and owning none of it
		{1, "0.00", "0.00"}, // an ordering, which MySQL does not price
		{3, "0.65", "0.65"}, // customers: prefix cost, and read plus eval
		{4, "2.60", "1.95"}, // orders
	}
	for _, tt := range tests {
		node := plan.Nodes[tt.at]
		cost, own := sqlkit.Cost(node.Cost), sqlkit.Cost(node.SelfCost)
		if cost != tt.cost || own != tt.own {
			t.Errorf("step %d (%s) cost %s of which %s its own, want %s and %s",
				tt.at, node.Type, cost, own, tt.cost, tt.own)
		}
	}
}

func TestReadPlanDetailNamesTheTableTheIndexAndTheFlags(t *testing.T) {
	plan := readPlan(planDocument)
	if plan == nil {
		t.Fatal("readPlan returned no plan")
	}

	tests := []struct {
		at   int
		want string
	}{
		{1, "using filesort"},
		{3, "on customers  filtered: 100.00%  (`c`.`country` = 'VN')"},
		{4, "on orders  using orders_customer_id_idx  filtered: 100.00%  using index"},
	}
	for _, tt := range tests {
		if got := plan.Nodes[tt.at].Detail; got != tt.want {
			t.Errorf("step %d detail = %q, want %q", tt.at, got, tt.want)
		}
	}
}

// The rows a step hands upward, which for a table is what the join produces and not what it scans.
func TestReadPlanReadsTheRowsAJoinProduces(t *testing.T) {
	plan := readPlan(planDocument)
	if plan == nil {
		t.Fatal("readPlan returned no plan")
	}
	if got := plan.Nodes[4].Rows; got != 6 {
		t.Errorf("orders produced %v rows, want the 6 of the join and not the 1 of the scan", got)
	}
}

func TestReadPlanSummaryCountsTheTablesAndTheStepsItRead(t *testing.T) {
	plan := readPlan(planDocument)
	if plan == nil {
		t.Fatal("readPlan returned no plan")
	}

	want := map[string]string{
		"Query cost":     "3.20",
		"Estimated rows": "6",
		"Steps":          "5",
		"Tables read":    "2",
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

func TestReadPlanRefusesADocumentThatIsNotOne(t *testing.T) {
	for _, document := range []string{"", "not json", "{}", `{"query_block": 3}`} {
		if plan := readPlan(document); plan != nil {
			t.Errorf("readPlan(%q) read a plan out of it", document)
		}
	}
}
