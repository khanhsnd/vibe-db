package sqlkit

import (
	"database/sql"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/driver"
)

func TestMergeSchemaNeedsEveryKeyColumnExactlyOnce(t *testing.T) {
	stored := []driver.Column{
		{Name: "id", FullType: "int unsigned", PrimaryKey: true},
		{Name: "name", FullType: "varchar(80)", Nullable: true},
	}

	tests := []struct {
		name   string
		result []string
		want   bool
	}{
		{name: "every column", result: []string{"id", "name"}, want: true},
		{name: "key alone", result: []string{"id"}, want: true},
		{name: "case insensitive", result: []string{"ID"}, want: true},
		{name: "key aliased away", result: []string{"name"}},
		{name: "key repeated", result: []string{"id", "id"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make([]driver.Column, 0, len(tt.result))
			for _, name := range tt.result {
				result = append(result, driver.Column{Name: name, Type: "INT"})
			}

			if got := MergeSchema(result, stored, []string{"id"}); got != tt.want {
				t.Fatalf("MergeSchema = %v, want %v", got, tt.want)
			}
			if !tt.want {
				return
			}
			if !result[0].PrimaryKey || result[0].FullType != "int unsigned" {
				t.Errorf("stored facts not copied onto %+v", result[0])
			}
		})
	}
}

func TestOrderBy(t *testing.T) {
	columns := []driver.Column{
		{Name: "id", PrimaryKey: true},
		{Name: "email"},
	}

	tests := []struct {
		name string
		sort []driver.TableSort
		want string
	}{
		{"no sort falls back to the primary key", nil, " ORDER BY `id`"},
		{"ascending", []driver.TableSort{{Column: "email"}}, " ORDER BY `email`"},
		{
			"descending",
			[]driver.TableSort{{Column: "email", Descending: true}},
			" ORDER BY `email` DESC",
		},
		{
			"the first key decides and the rest break its ties",
			[]driver.TableSort{{Column: "email", Descending: true}, {Column: "id"}},
			" ORDER BY `email` DESC, `id`",
		},
		{
			"a column the table has not got is dropped",
			[]driver.TableSort{{Column: "1; DROP TABLE users"}},
			" ORDER BY `id`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MySQL.OrderBy(tt.sort, columns); got != tt.want {
				t.Errorf("OrderBy = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOrderByWithoutPrimaryKeyOrSortIsEmpty(t *testing.T) {
	got := MySQL.OrderBy(nil, []driver.Column{{Name: "email"}})

	if got != "" {
		t.Errorf("OrderBy = %q, want no clause", got)
	}
}

func TestPrimaryKeyNamesKeepsOrdinalOrder(t *testing.T) {
	columns := []driver.Column{
		{Name: "order_id", PrimaryKey: true},
		{Name: "quantity"},
		{Name: "sku", PrimaryKey: true},
	}

	got := PrimaryKeyNames(columns)

	if len(got) != 2 || got[0] != "order_id" || got[1] != "sku" {
		t.Errorf("PrimaryKeyNames = %v, want [order_id sku]", got)
	}
}

func TestAppendColumnGroupsConsecutiveRows(t *testing.T) {
	rows := [][2]string{
		{"users", "id"},
		{"users", "email"},
		{"orders", "id"},
	}

	tables := []driver.TableSchema{}
	for _, row := range rows {
		tables = AppendColumn(tables, row[0], row[1])
	}

	if len(tables) != 2 {
		t.Fatalf("len(tables) = %d, want 2", len(tables))
	}
	if tables[0].Name != "users" || len(tables[0].Columns) != 2 {
		t.Errorf("tables[0] = %+v, want users with 2 columns", tables[0])
	}
	if tables[0].Columns[0] != "id" || tables[0].Columns[1] != "email" {
		t.Error("columns must keep ordinal order, not be sorted")
	}
	if tables[1].Name != "orders" {
		t.Errorf("tables[1].Name = %q, want orders", tables[1].Name)
	}
}

func TestTextOrNilKeepsTextAndHexesBytes(t *testing.T) {
	tests := []struct {
		name  string
		value sql.NullString
		want  *string
	}{
		{"a NULL", sql.NullString{}, nil},
		{"the empty string", sql.NullString{String: "", Valid: true}, ptr("")},
		{"plain text", sql.NullString{String: "hello", Valid: true}, ptr("hello")},
		{
			// Text that is not ASCII is still text, and must not be turned into hex: a column of
			// Vietnamese names is the ordinary case, not a blob.
			"text outside ASCII",
			sql.NullString{String: "Nguyễn — 日本", Valid: true},
			ptr("Nguyễn — 日本"),
		},
		{
			// A PNG's first bytes are not valid UTF-8, and are exactly what the WebView would
			// otherwise replace character by character on its way across.
			"bytes that are not text",
			sql.NullString{String: "\x89PNG\r\n\x1a\n", Valid: true},
			ptr(`\x89504e470d0a1a0a`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TextOrNil(tt.value)

			if tt.want == nil {
				if got != nil {
					t.Fatalf("TextOrNil() = %q, want nil", *got)
				}
				return
			}
			if got == nil {
				t.Fatalf("TextOrNil() = nil, want %q", *tt.want)
			}
			if *got != *tt.want {
				t.Errorf("TextOrNil() = %q, want %q", *got, *tt.want)
			}
		})
	}
}

func ptr(text string) *string {
	return &text
}
