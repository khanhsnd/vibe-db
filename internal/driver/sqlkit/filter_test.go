package sqlkit

import (
	"errors"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

var filterColumns = []driver.Column{{Name: "id"}, {Name: "email"}}

func TestWhereWithNoFiltersIsEmpty(t *testing.T) {
	clause, err := MySQL.Where(nil, filterColumns)
	if err != nil {
		t.Fatalf("Where() error = %v", err)
	}
	if clause != "" {
		t.Errorf("Where() = %q, want empty", clause)
	}
}

func TestWhereSpellsEveryOperator(t *testing.T) {
	cases := []struct {
		op   driver.TableFilterOp
		want string
	}{
		{driver.FilterEqual, " WHERE `email` = 'a@b.c'"},
		{driver.FilterNotEqual, " WHERE `email` <> 'a@b.c'"},
		{driver.FilterGreater, " WHERE `email` > 'a@b.c'"},
		{driver.FilterGreaterEqual, " WHERE `email` >= 'a@b.c'"},
		{driver.FilterLess, " WHERE `email` < 'a@b.c'"},
		{driver.FilterLessEqual, " WHERE `email` <= 'a@b.c'"},
		{driver.FilterContains, " WHERE CAST(`email` AS CHAR) LIKE '%a@b.c%'"},
	}

	for _, one := range cases {
		filters := []driver.TableFilter{{Column: "email", Op: one.op, Value: "a@b.c"}}

		got, err := MySQL.Where(filters, filterColumns)
		if err != nil {
			t.Fatalf("Where(%s) error = %v", one.op, err)
		}
		if got != one.want {
			t.Errorf("Where(%s) = %q, want %q", one.op, got, one.want)
		}
	}
}

func TestWhereNullOperatorsTakeNoValue(t *testing.T) {
	filters := []driver.TableFilter{
		{Column: "email", Op: driver.FilterNull, Value: "ignored"},
		{Column: "id", Op: driver.FilterNotNull},
	}

	got, err := Postgres.Where(filters, filterColumns)
	if err != nil {
		t.Fatalf("Where() error = %v", err)
	}

	want := ` WHERE "email" IS NULL AND "id" IS NOT NULL`
	if got != want {
		t.Errorf("Where() = %q, want %q", got, want)
	}
}

func TestWhereQuotesAValueThatCarriesAQuote(t *testing.T) {
	filters := []driver.TableFilter{{Column: "email", Op: driver.FilterEqual, Value: "o'brien"}}

	got, err := Postgres.Where(filters, filterColumns)
	if err != nil {
		t.Fatalf("Where() error = %v", err)
	}

	want := ` WHERE "email" = 'o''brien'`
	if got != want {
		t.Errorf("Where() = %q, want %q", got, want)
	}
}

func TestWhereRefusesAnUnknownColumn(t *testing.T) {
	filters := []driver.TableFilter{{Column: "nope", Op: driver.FilterEqual, Value: "1"}}

	if _, err := MySQL.Where(filters, filterColumns); !isValidation(err) {
		t.Errorf("Where() error = %v, want a validation error", err)
	}
}

func TestWhereRefusesAnUnknownOperator(t *testing.T) {
	filters := []driver.TableFilter{{Column: "id", Op: "; DROP TABLE users", Value: "1"}}

	if _, err := MySQL.Where(filters, filterColumns); !isValidation(err) {
		t.Errorf("Where() error = %v, want a validation error", err)
	}
}

func isValidation(err error) bool {
	var validation domain.ValidationError
	return errors.As(err, &validation)
}
