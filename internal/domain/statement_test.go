package domain

import "testing"

func TestStatementHazard(t *testing.T) {
	tests := []struct {
		keyword  string
		filtered bool
		want     Hazard
	}{
		{"select", false, HazardNone},
		{"explain", false, HazardNone},
		{"show", false, HazardNone},
		{"with", false, HazardNone},
		{"insert", false, HazardWrite},
		{"create", false, HazardWrite},
		{"drop", false, HazardDestructive},
		{"truncate", false, HazardDestructive},
		{"alter", false, HazardDestructive},
		{"delete", true, HazardWrite},
		{"delete", false, HazardUnfiltered},
		{"update", true, HazardWrite},
		{"update", false, HazardUnfiltered},
	}

	for _, tt := range tests {
		if got := StatementHazard(tt.keyword, tt.filtered); got != tt.want {
			t.Errorf("StatementHazard(%q, %v) = %q, want %q", tt.keyword, tt.filtered, got, tt.want)
		}
	}
}

func TestNeedsConfirmation(t *testing.T) {
	tests := []struct {
		environment Environment
		hazard      Hazard
		want        bool
	}{
		// Nothing is ever confirmed for a read, wherever it runs.
		{EnvironmentProduction, HazardNone, false},
		{EnvironmentDevelopment, HazardNone, false},

		// An unguarded environment waves a write through and still stops an unfiltered one.
		{EnvironmentDevelopment, HazardWrite, false},
		{EnvironmentDevelopment, HazardDestructive, false},
		{EnvironmentDevelopment, HazardUnfiltered, true},
		{EnvironmentUnset, HazardUnfiltered, true},
		{EnvironmentStaging, HazardDestructive, false},

		// Production answers for all three.
		{EnvironmentProduction, HazardWrite, true},
		{EnvironmentProduction, HazardUnfiltered, true},
		{EnvironmentProduction, HazardDestructive, true},
	}

	for _, tt := range tests {
		got := tt.environment.NeedsConfirmation(tt.hazard)
		if got != tt.want {
			t.Errorf("%q.NeedsConfirmation(%q) = %v, want %v",
				tt.environment, tt.hazard, got, tt.want)
		}
	}
}

func TestOnlyProductionIsGuarded(t *testing.T) {
	guarded := map[Environment]bool{
		EnvironmentUnset:       false,
		EnvironmentDevelopment: false,
		EnvironmentStaging:     false,
		EnvironmentProduction:  true,
	}

	for environment, want := range guarded {
		if got := environment.Guarded(); got != want {
			t.Errorf("%q.Guarded() = %v, want %v", environment, got, want)
		}
	}
}
