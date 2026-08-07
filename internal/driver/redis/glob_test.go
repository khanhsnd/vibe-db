package redis

import "testing"

func TestGlobMatchAnswersWhatTheServerWouldHaveAnswered(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		value   string
		want    bool
	}{
		{"a run", "*email*", "user:email:1", true},
		{"a run that is not there", "*email*", "user:name:1", false},
		{"a run reaches nothing as well as something", "user:*", "user:", true},
		{"a star is the whole key space", "*", "anything", true},
		{"a run of stars says no more than one", "**user**", "a:user:1", true},
		{"one character", "user:?", "user:1", true},
		{"and exactly one", "user:?", "user:12", false},
		{"a class", "user:[12]", "user:2", true},
		{"a class it is not in", "user:[12]", "user:3", false},
		{"a negated class", "user:[^12]", "user:3", true},
		{"a range", "user:[0-9]", "user:7", true},
		{"a range it is above", "user:[0-9]", "user:a", false},
		{"an escaped star is a star", `user:\*`, "user:*", true},
		{"and not a run", `user:\*`, "user:1", false},
		{"a key with no colon", "email", "email", true},
		{"a pattern that runs past the key", "user:1*", "user:", false},
		// A key is bytes, so a pattern that matches half of one matches nothing rather than crashing.
		{"one character of two bytes", "?", "é", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := globMatch(tt.pattern, tt.value); got != tt.want {
				t.Errorf("globMatch(%q, %q) = %v, want %v", tt.pattern, tt.value, got, tt.want)
			}
		})
	}
}

// The two spellings have to agree: the server narrows the scan by the glob and the tree is then
// applied here to what came back, so a key the glob admits must be a key this admits too.
func TestFoldedContainsIsTheSameQuestionSpeltForTheServer(t *testing.T) {
	if got, want := foldGlob("Us1"), "[uU][sS]1"; got != want {
		t.Errorf("foldGlob() = %q, want %q", got, want)
	}

	for _, key := range []string{"user:1", "USER:1", "User:1"} {
		if !globMatch("*"+foldGlob("uSeR")+"*", key) {
			t.Errorf("the glob for a folded contains missed %q", key)
		}
	}
}
