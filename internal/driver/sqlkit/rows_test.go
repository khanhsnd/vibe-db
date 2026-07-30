package sqlkit

import (
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
