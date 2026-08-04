package sqlkit

import (
	"errors"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

var filterColumns = []driver.Column{{Name: "id"}, {Name: "email"}}

// and is the root a read carries when nothing has said otherwise, which is every case below
// that is not about a group.
func and(children ...driver.TableCriterion) driver.TableGroup {
	return driver.TableGroup{Conjunction: driver.ConjunctionAnd, Children: children}
}

func TestWhereWithNoFiltersIsEmpty(t *testing.T) {
	clause, err := MySQL.Where(driver.TableGroup{}, filterColumns)
	if err != nil {
		t.Fatalf("Where() error = %v", err)
	}
	if clause != "" {
		t.Errorf("Where() = %q, want empty", clause)
	}
}

func TestWhereSpellsEveryOperator(t *testing.T) {
	cases := []struct {
		op     driver.TableFilterOp
		values []string
		want   string
	}{
		{driver.FilterEqual, []string{"a@b.c"}, " WHERE `email` = 'a@b.c'"},
		{driver.FilterNotEqual, []string{"a@b.c"}, " WHERE `email` <> 'a@b.c'"},
		{driver.FilterGreater, []string{"a@b.c"}, " WHERE `email` > 'a@b.c'"},
		{driver.FilterGreaterEqual, []string{"a@b.c"}, " WHERE `email` >= 'a@b.c'"},
		{driver.FilterLess, []string{"a@b.c"}, " WHERE `email` < 'a@b.c'"},
		{driver.FilterLessEqual, []string{"a@b.c"}, " WHERE `email` <= 'a@b.c'"},
		{
			driver.FilterContains,
			[]string{"shop"},
			" WHERE CAST(`email` AS CHAR) LIKE '%shop%'",
		},
		{
			driver.FilterNotContains,
			[]string{"shop"},
			" WHERE CAST(`email` AS CHAR) NOT LIKE '%shop%'",
		},
		{driver.FilterBegins, []string{"a"}, " WHERE CAST(`email` AS CHAR) LIKE 'a%'"},
		{driver.FilterNotBegins, []string{"a"}, " WHERE CAST(`email` AS CHAR) NOT LIKE 'a%'"},
		{driver.FilterEnds, []string{".c"}, " WHERE CAST(`email` AS CHAR) LIKE '%.c'"},
		{driver.FilterNotEnds, []string{".c"}, " WHERE CAST(`email` AS CHAR) NOT LIKE '%.c'"},
		{driver.FilterNull, nil, " WHERE `email` IS NULL"},
		{driver.FilterNotNull, nil, " WHERE `email` IS NOT NULL"},
		{driver.FilterEmpty, nil, " WHERE `email` = ''"},
		{driver.FilterNotEmpty, nil, " WHERE `email` <> ''"},
		{
			driver.FilterBetween,
			[]string{"a", "z"},
			" WHERE `email` BETWEEN 'a' AND 'z'",
		},
		{
			driver.FilterNotBetween,
			[]string{"a", "z"},
			" WHERE `email` NOT BETWEEN 'a' AND 'z'",
		},
		{driver.FilterInList, []string{"a", "b"}, " WHERE `email` IN ('a', 'b')"},
		{driver.FilterNotInList, []string{"a"}, " WHERE `email` NOT IN ('a')"},
	}

	for _, one := range cases {
		filter := driver.TableFilter{Column: "email", Op: one.op, Values: one.values}

		got, err := MySQL.Where(and(filter), filterColumns)
		if err != nil {
			t.Fatalf("Where(%s) error = %v", one.op, err)
		}
		if got != one.want {
			t.Errorf("Where(%s) = %q, want %q", one.op, got, one.want)
		}
	}
}

func TestWhereJoinsSiblingsWithTheirOwnConjunction(t *testing.T) {
	filter := and(
		driver.TableFilter{Column: "email", Op: driver.FilterNull},
		driver.TableFilter{Column: "id", Op: driver.FilterNotNull},
	)

	got, err := Postgres.Where(filter, filterColumns)
	if err != nil {
		t.Fatalf("Where() error = %v", err)
	}

	want := ` WHERE "email" IS NULL AND "id" IS NOT NULL`
	if got != want {
		t.Errorf("Where() = %q, want %q", got, want)
	}
}

// A nested group is what parentheses are for: `a AND (b OR c)` cannot be said in a flat list.
func TestWhereParenthesisesANestedGroup(t *testing.T) {
	filter := and(
		driver.TableFilter{Column: "id", Op: driver.FilterGreater, Values: []string{"10"}},
		driver.TableGroup{
			Conjunction: driver.ConjunctionOr,
			Children: []driver.TableCriterion{
				driver.TableFilter{Column: "email", Op: driver.FilterNull},
				driver.TableFilter{Column: "email", Op: driver.FilterEmpty},
			},
		},
	)

	got, err := MySQL.Where(filter, filterColumns)
	if err != nil {
		t.Fatalf("Where() error = %v", err)
	}

	want := " WHERE `id` > '10' AND (`email` IS NULL OR `email` = '')"
	if got != want {
		t.Errorf("Where() = %q, want %q", got, want)
	}
}

// An empty group contributes nothing rather than `()`, so a reader who added a group and has not
// filled it in yet still reads the rows the rest of the tree admits.
func TestWhereDropsAnEmptyGroup(t *testing.T) {
	filter := and(
		driver.TableFilter{Column: "id", Op: driver.FilterNotNull},
		driver.TableGroup{Conjunction: driver.ConjunctionOr},
	)

	got, err := MySQL.Where(filter, filterColumns)
	if err != nil {
		t.Fatalf("Where() error = %v", err)
	}

	want := " WHERE `id` IS NOT NULL"
	if got != want {
		t.Errorf("Where() = %q, want %q", got, want)
	}
}

func TestWhereQuotesAValueThatCarriesAQuote(t *testing.T) {
	filter := and(driver.TableFilter{
		Column: "email",
		Op:     driver.FilterEqual,
		Values: []string{"o'brien"},
	})

	got, err := Postgres.Where(filter, filterColumns)
	if err != nil {
		t.Fatalf("Where() error = %v", err)
	}

	want := ` WHERE "email" = 'o''brien'`
	if got != want {
		t.Errorf("Where() = %q, want %q", got, want)
	}
}

func TestWhereRefusesAnUnknownColumn(t *testing.T) {
	filter := and(driver.TableFilter{
		Column: "nope",
		Op:     driver.FilterEqual,
		Values: []string{"1"},
	})

	if _, err := MySQL.Where(filter, filterColumns); !isValidation(err) {
		t.Errorf("Where() error = %v, want a validation error", err)
	}
}

func TestWhereRefusesAnUnknownOperator(t *testing.T) {
	filter := and(driver.TableFilter{
		Column: "id",
		Op:     "; DROP TABLE users",
		Values: []string{"1"},
	})

	if _, err := MySQL.Where(filter, filterColumns); !isValidation(err) {
		t.Errorf("Where() error = %v, want a validation error", err)
	}
}

func TestWhereRefusesAnUnknownConjunction(t *testing.T) {
	filter := driver.TableGroup{
		Conjunction: "; DROP TABLE users",
		Children: []driver.TableCriterion{
			driver.TableFilter{Column: "id", Op: driver.FilterNotNull},
			driver.TableFilter{Column: "email", Op: driver.FilterNotNull},
		},
	}

	if _, err := MySQL.Where(filter, filterColumns); !isValidation(err) {
		t.Errorf("Where() error = %v, want a validation error", err)
	}
}

// Half a BETWEEN is not SQL, and a list of nothing is not a comparison either.
func TestWhereRefusesTheWrongNumberOfValues(t *testing.T) {
	cases := []driver.TableFilter{
		{Column: "id", Op: driver.FilterEqual},
		{Column: "id", Op: driver.FilterBetween, Values: []string{"1"}},
		{Column: "id", Op: driver.FilterInList},
	}

	for _, filter := range cases {
		if _, err := MySQL.Where(and(filter), filterColumns); !isValidation(err) {
			t.Errorf("Where(%s) error = %v, want a validation error", filter.Op, err)
		}
	}
}

func TestDistinctValuesReadsOneColumn(t *testing.T) {
	got, err := MySQL.DistinctValues("`shop`.`users`", "email", filterColumns, 200)
	if err != nil {
		t.Fatalf("DistinctValues() error = %v", err)
	}

	want := "SELECT DISTINCT `email` FROM `shop`.`users`" +
		" WHERE `email` IS NOT NULL ORDER BY 1 LIMIT 200"
	if got != want {
		t.Errorf("DistinctValues() = %q, want %q", got, want)
	}
}

func TestDistinctValuesRefusesAnUnknownColumn(t *testing.T) {
	_, err := MySQL.DistinctValues("`shop`.`users`", "nope", filterColumns, 200)

	if !isValidation(err) {
		t.Errorf("DistinctValues() error = %v, want a validation error", err)
	}
}

func isValidation(err error) bool {
	var validation domain.ValidationError
	return errors.As(err, &validation)
}
