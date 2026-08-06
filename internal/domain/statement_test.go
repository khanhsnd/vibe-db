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

// The same rule read off a different grammar: a document store keeps its verb in the method, and
// what it reaches keeps depending on whether anything narrowed it.
func TestMongoHazardReadsTheMethodAndItsFilter(t *testing.T) {
	tests := []struct {
		method   string
		filtered bool
		want     Hazard
	}{
		{"find", false, HazardNone},
		{"aggregate", false, HazardNone},
		{"countDocuments", false, HazardNone},
		{"insertOne", false, HazardWrite},
		{"createIndex", false, HazardWrite},
		{"updateOne", true, HazardWrite},
		{"deleteMany", true, HazardWrite},
		{"updateMany", false, HazardUnfiltered},
		{"deleteMany", false, HazardUnfiltered},
		{"drop", false, HazardDestructive},
		{"dropDatabase", false, HazardDestructive},
		{"renameCollection", false, HazardDestructive},

		// The server's own spellings, which is what `db.runCommand({ … })` carries.
		{"create", false, HazardWrite},
		{"insert", false, HazardWrite},
		{"delete", false, HazardUnfiltered},
		{"ping", false, HazardNone},

		// A method that reads is silent, and so is one this side cannot tell apart from a read.
		{"frobnicate", false, HazardNone},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			if got := MongoHazard(tt.method, tt.filtered); got != tt.want {
				t.Errorf("MongoHazard(%q, %v) = %q, want %q", tt.method, tt.filtered, got, tt.want)
			}
		})
	}
}

// The method is named however the reader typed it, so a rule read off it cannot depend on the case.
func TestMongoHazardIgnoresCase(t *testing.T) {
	if got := MongoHazard("DROPDATABASE", false); got != HazardDestructive {
		t.Errorf("MongoHazard() = %q, want %q", got, HazardDestructive)
	}
}

func TestRedisHazardReadsTheCommandAndItsContainer(t *testing.T) {
	tests := []struct {
		command  string
		argument string
		want     Hazard
	}{
		{"GET", "user:1", HazardNone},
		{"SCAN", "0", HazardNone},
		{"INFO", "keyspace", HazardNone},
		{"KEYS", "*", HazardNone},

		{"SET", "user:1", HazardWrite},
		{"DEL", "user:1", HazardWrite},
		{"UNLINK", "user:1", HazardWrite},
		{"HSET", "user:1", HazardWrite},
		{"EXPIRE", "user:1", HazardWrite},
		{"EVAL", "return 1", HazardWrite},

		{"RENAME", "a", HazardDestructive},
		{"SHUTDOWN", "", HazardDestructive},

		// A container command keeps its verb in two words, so one word is not enough to tell the
		// two apart.
		{"CONFIG", "GET", HazardNone},
		{"CONFIG", "SET", HazardDestructive},
		{"CLIENT", "LIST", HazardNone},
		{"CLIENT", "KILL", HazardDestructive},

		// The commands that name nothing and therefore take everything. They are answered for
		// wherever they are run, which is what being unfiltered means here.
		{"FLUSHDB", "", HazardUnfiltered},
		{"FLUSHALL", "ASYNC", HazardUnfiltered},
		{"SCRIPT", "FLUSH", HazardUnfiltered},
	}

	for _, tt := range tests {
		t.Run(tt.command+" "+tt.argument, func(t *testing.T) {
			if got := RedisHazard(tt.command, tt.argument); got != tt.want {
				t.Errorf("RedisHazard(%q, %q) = %q, want %q", tt.command, tt.argument, got, tt.want)
			}
		})
	}
}

func TestRedisHazardIgnoresCase(t *testing.T) {
	if got := RedisHazard("flushdb", ""); got != HazardUnfiltered {
		t.Errorf("RedisHazard(\"flushdb\") = %q, want %q", got, HazardUnfiltered)
	}
	if got := RedisHazard("config", "set"); got != HazardDestructive {
		t.Errorf("RedisHazard(\"config\", \"set\") = %q, want %q", got, HazardDestructive)
	}
}

// Anything that takes everything is confirmed wherever it runs, and that is the whole reason
// FLUSHDB is classified unfiltered rather than destructive.
func TestAFlushIsConfirmedOnALaptopToo(t *testing.T) {
	flush := RedisHazard("FLUSHALL", "")

	if !EnvironmentDevelopment.NeedsConfirmation(flush) {
		t.Error("a flush on development is not confirmed")
	}
	if !EnvironmentProduction.NeedsConfirmation(flush) {
		t.Error("a flush on production is not confirmed")
	}
}
