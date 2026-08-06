package redis

import (
	"strings"
	"testing"
)

func TestJSONIsWrittenInAnOrderThatDoesNotMove(t *testing.T) {
	if got := jsonArray([]string{"a", `b"c`, ""}); got != `["a","b\"c",""]` {
		t.Errorf("jsonArray() = %s, want the escaping to be the encoder's", got)
	}
	if got := jsonPairs([][2]string{{"z", "1"}, {"a", "2"}}); got != `{"z":"1","a":"2"}` {
		t.Errorf("jsonPairs() = %s, want the pairs in the order given", got)
	}
}

func TestParseValueReadsAContainerBackIntoCommands(t *testing.T) {
	tests := []struct {
		name  string
		kind  string
		value string
		want  []write
	}{
		{"a list", typeList, `["a","b"]`, []write{{name: "RPUSH", args: []string{"a", "b"}}}},
		{"a set", typeSet, `["a"]`, []write{{name: "SADD", args: []string{"a"}}}},
		{"a hash", typeHash, `{"n":"1"}`, []write{{name: "HSET", args: []string{"n", "1"}}}},
		{"a sorted set", typeZSet, `{"bob":"1.5"}`, []write{{name: "ZADD", args: []string{"1.5", "bob"}}}},
		{
			// A member written as a number is the text of one, because that is what the server would
			// store either way.
			"a number as a member",
			typeList,
			`[1,true,null]`,
			[]write{{name: "RPUSH", args: []string{"1", "true", ""}}},
		},
		{"an empty container", typeList, `[]`, []write{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseValue(tt.kind, tt.value)
			if err != nil {
				t.Fatalf("parseValue() error = %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("parseValue() = %+v, want %+v", got, tt.want)
			}
			for i := range got {
				if got[i].name != tt.want[i].name {
					t.Errorf("command %d = %q, want %q", i, got[i].name, tt.want[i].name)
				}
				if strings.Join(got[i].args, " ") != strings.Join(tt.want[i].args, " ") {
					t.Errorf("arguments %d = %q, want %q", i, got[i].args, tt.want[i].args)
				}
			}
		})
	}
}

func TestParseValueRefusesWhatItCannotWriteBack(t *testing.T) {
	refusals := []struct {
		name  string
		kind  string
		value string
	}{
		{"a list that is not JSON", typeList, `a,b,c`},
		{"a hash that is an array", typeHash, `["a"]`},
		{"a score that is not a number", typeZSet, `{"bob":"soon"}`},
		{"a member that is itself a list", typeList, `[["a"]]`},
		{"a stream, which the server numbers", typeStream, `[]`},
		{"a type from a module", "MBbloom--", `{}`},
	}

	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseValue(tt.kind, tt.value); err == nil {
				t.Error("parseValue() error = nil, want a refusal")
			}
		})
	}
}

// A value the grid could only sample must not be written back: whatever it did not show would go
// with it. The mark is what makes that edit fail, and it fails before anything is sent anywhere.
func TestASampledValueCannotBeWrittenBack(t *testing.T) {
	shown := sampled(`["a","b"]`, 2, 4000)
	if !strings.Contains(shown, sampleMark) {
		t.Fatalf("sampled() = %q, want it marked", shown)
	}

	err := writeValue(t.Context(), nil, "queue", typeList, shown)
	if err == nil {
		t.Fatal("writeValue() error = nil, want a refusal")
	}
	if !strings.Contains(err.Error(), "sample") {
		t.Errorf("error = %q, want it to say the value is a sample", err)
	}
}

func TestAValueShownWholeIsNotMarked(t *testing.T) {
	if got := sampled(`["a","b"]`, 2, 2); got != `["a","b"]` {
		t.Errorf("sampled() = %q, want the value itself", got)
	}
}
