package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

func TestProfileInputEngineDecidesTheDefaultPort(t *testing.T) {
	tests := []struct {
		name       string
		in         ProfileInput
		wantEngine domain.Engine
		wantPort   int
	}{
		{
			name:       "a blank engine is MySQL, as a profile written before the second engine means",
			in:         ProfileInput{Name: "local"},
			wantEngine: domain.EngineMySQL,
			wantPort:   domain.DefaultMySQLPort,
		},
		{
			name:       "postgres without a port",
			in:         ProfileInput{Name: "local", Engine: "postgres"},
			wantEngine: domain.EnginePostgres,
			wantPort:   domain.DefaultPostgresPort,
		},
		{
			name:       "a named port wins over the engine default",
			in:         ProfileInput{Name: "local", Engine: "postgres", Port: 6543},
			wantEngine: domain.EnginePostgres,
			wantPort:   6543,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.in.toDomain()

			if got.Engine != tt.wantEngine {
				t.Errorf("Engine = %q, want %q", got.Engine, tt.wantEngine)
			}
			if got.Port != tt.wantPort {
				t.Errorf("Port = %d, want %d", got.Port, tt.wantPort)
			}
		})
	}
}

func TestQueryInputMaxRows(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{"zero falls back to the default", 0, DefaultQueryRows},
		{"negative falls back to the default", -5, DefaultQueryRows},
		{"a sane value is kept", 250, 250},
		{"a large typed value is not clamped", 250_000, 250_000},
		{"all becomes the driver's uncapped read", AllRows, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := QueryInput{SQL: "SELECT 1", MaxRows: tt.in}.toDriver()

			if got.MaxRows != tt.want {
				t.Errorf("MaxRows = %d, want %d", got.MaxRows, tt.want)
			}
		})
	}
}

func TestTablePageInputLimit(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{"zero falls back to the default", 0, DefaultPageSize},
		{"negative falls back to the default", -5, DefaultPageSize},
		{"a sane value is kept", 250, 250},
		{"a large typed value is not clamped", 250_000, 250_000},
		{"all becomes the driver's uncapped read", AllRows, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TablePageInput{Table: "users", Limit: tt.in}.toDriver()

			if got.Limit != tt.want {
				t.Errorf("Limit = %d, want %d", got.Limit, tt.want)
			}
		})
	}
}

func TestTablePageInputClampsNegativeOffset(t *testing.T) {
	got := TablePageInput{Table: "users", Limit: 100, Offset: -1}.toDriver()

	if got.Offset != 0 {
		t.Errorf("Offset = %d, want 0", got.Offset)
	}
}

// The wire carries the condition tree flat, one struct standing for either kind of node, and this
// is where it becomes the two types the driver switches on.
func TestTableCriterionInputBecomesATree(t *testing.T) {
	in := TableCriterionInput{
		Kind:        criterionGroup,
		Conjunction: "and",
		Children: []TableCriterionInput{
			{Kind: "condition", Column: "email", Operator: "contains", Values: []string{"shop"}},
			{
				Kind:        criterionGroup,
				Conjunction: "or",
				Children: []TableCriterionInput{
					{Kind: "condition", Column: "age", Operator: "between", Values: []string{"1", "5"}},
				},
			},
		},
	}

	root := in.toDriverGroup()
	if root.Conjunction != driver.ConjunctionAnd || len(root.Children) != 2 {
		t.Fatalf("root = %+v", root)
	}

	first, ok := root.Children[0].(driver.TableFilter)
	if !ok || first.Op != driver.FilterContains || len(first.Values) != 1 {
		t.Errorf("first child = %+v", root.Children[0])
	}

	nested, ok := root.Children[1].(driver.TableGroup)
	if !ok || nested.Conjunction != driver.ConjunctionOr || len(nested.Children) != 1 {
		t.Fatalf("second child = %+v", root.Children[1])
	}

	inner, ok := nested.Children[0].(driver.TableFilter)
	if !ok || inner.Op != driver.FilterBetween || len(inner.Values) != 2 {
		t.Errorf("nested child = %+v", nested.Children[0])
	}
}

// A read that carried no filter carries the zero group, which the driver reads as narrowing
// nothing — so a wire node this side cannot name must come out that way rather than as a condition
// on no column.
func TestTablePageInputWithoutAFilterNarrowsNothing(t *testing.T) {
	got := TablePageInput{Table: "users", Limit: 100}.toDriver()

	if len(got.Filter.Children) != 0 {
		t.Errorf("Filter = %+v, want the group of nothing", got.Filter)
	}
}

func TestTablePageInputKeepsEverySortKeyInOrder(t *testing.T) {
	got := TablePageInput{
		Table: "users",
		Limit: 100,
		Sort: []TableSortInput{
			{Column: "name", Descending: true},
			{Column: "id"},
		},
	}.toDriver()

	want := []driver.TableSort{{Column: "name", Descending: true}, {Column: "id"}}
	if len(got.Sort) != len(want) || got.Sort[0] != want[0] || got.Sort[1] != want[1] {
		t.Errorf("Sort = %+v, want %+v", got.Sort, want)
	}
}

func TestQueryResultDTOSendsListsAndNeverNull(t *testing.T) {
	blob, err := json.Marshal(toQueryResultDTO(driver.QueryResult{}, 0))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(blob), `"rows":null`) || strings.Contains(string(blob), `"columns":null`) {
		t.Fatalf("columns and rows must cross the wire as lists, got %s", blob)
	}
}

func TestScriptResultOfStatementsThatReturnedNothing(t *testing.T) {
	script := newScriptResult(2)
	script.Statements = append(script.Statements,
		StatementOutcome{SQL: "SET @id = 1"},
		StatementOutcome{SQL: "UPDATE t SET a = 1 WHERE b = 2"},
	)

	blob, err := json.Marshal(script)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(blob), `"rows":null`) || strings.Contains(string(blob), `"columns":null`) {
		t.Fatalf("a script whose statements returned no rows still answers with an empty result, got %s", blob)
	}
}
