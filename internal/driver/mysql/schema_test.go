package mysql

import (
	"testing"

	"github.com/khanhsnd/vibe-db/internal/driver"
)

func TestAppendColumnGroupsConsecutiveRows(t *testing.T) {
	rows := [][2]string{
		{"users", "id"},
		{"users", "email"},
		{"orders", "id"},
	}

	tables := []driver.TableSchema{}
	for _, row := range rows {
		tables = appendColumn(tables, row[0], row[1])
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
