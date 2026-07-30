package app

import "testing"

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
