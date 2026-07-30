package app

import "testing"

func TestQueryInputClampsMaxRows(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{"zero falls back to the default", 0, DefaultQueryRows},
		{"negative falls back to the default", -1, DefaultQueryRows},
		{"a sane value is kept", 250, 250},
		{"over the ceiling is capped", MaxQueryRows * 2, MaxQueryRows},
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
