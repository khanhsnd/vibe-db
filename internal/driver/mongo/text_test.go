package mongo

import (
	"strings"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

func TestReviewReadsWhatEachCommandWillDo(t *testing.T) {
	tests := []struct {
		mql  string
		want domain.Hazard
	}{
		{`db.users.find({})`, domain.HazardNone},
		{`db.users.findOne({ a: 1 })`, domain.HazardNone},
		{`db.users.aggregate([{ $match: {} }])`, domain.HazardNone},
		{`db.users.countDocuments({})`, domain.HazardNone},
		{`db.users.distinct("tier")`, domain.HazardNone},
		{`db.users.getIndexes()`, domain.HazardNone},
		{`db.users.insertOne({ a: 1 })`, domain.HazardWrite},
		{`db.users.createIndex({ a: 1 })`, domain.HazardWrite},
		{`db.users.updateOne({ a: 1 }, { $set: { b: 2 } })`, domain.HazardWrite},
		{`db.users.deleteOne({ a: 1 })`, domain.HazardWrite},

		// The filter is what decides these, exactly as a WHERE decides an UPDATE: with none, each
		// of them takes every document in the collection.
		{`db.users.updateMany({}, { $set: { b: 2 } })`, domain.HazardUnfiltered},
		{`db.users.deleteMany({})`, domain.HazardUnfiltered},
		{`db.users.deleteMany()`, domain.HazardUnfiltered},
		{`db.users.deleteMany({ a: 1 })`, domain.HazardWrite},

		{`db.users.drop()`, domain.HazardDestructive},
		{`db.users.dropIndex("a_1")`, domain.HazardDestructive},
		{`db.dropDatabase()`, domain.HazardDestructive},
	}

	for _, tt := range tests {
		t.Run(tt.mql, func(t *testing.T) {
			review := text{}.Review(tt.mql)
			if len(review.Statements) != 1 {
				t.Fatalf("Review() = %d statements, want 1", len(review.Statements))
			}
			if got := review.Statements[0].Hazard; got != tt.want {
				t.Errorf("Hazard = %q, want %q", got, tt.want)
			}
			if len(review.Problems) != 0 {
				t.Errorf("Problems = %v, want none", review.Problems)
			}
		})
	}
}

// `runCommand` is whatever it is told to be, so what it is answered for is the command inside it and
// not the name of the method carrying it.
func TestReviewReadsRunCommandsOwnCommand(t *testing.T) {
	tests := []struct {
		mql     string
		keyword string
		want    domain.Hazard
	}{
		{`db.runCommand({ ping: 1 })`, "ping", domain.HazardNone},
		{`db.runCommand({ drop: "users" })`, "drop", domain.HazardDestructive},
		{`db.runCommand({ dropDatabase: 1 })`, "dropDatabase", domain.HazardDestructive},
		{`db.runCommand({ create: "users" })`, "create", domain.HazardWrite},
	}

	for _, tt := range tests {
		t.Run(tt.mql, func(t *testing.T) {
			review := text{}.Review(tt.mql)
			one := review.Statements[0]

			if one.Keyword != tt.keyword {
				t.Errorf("Keyword = %q, want %q", one.Keyword, tt.keyword)
			}
			if one.Hazard != tt.want {
				t.Errorf("Hazard = %q, want %q", one.Hazard, tt.want)
			}
		})
	}
}

// Every command is placed in the script it came from, which is what lets a warning point at it
// rather than repeat it.
func TestReviewSaysWhereEachCommandSits(t *testing.T) {
	const script = "db.users.find({});\n\ndb.users.deleteMany({});\n"
	review := text{}.Review(script)

	if len(review.Statements) != 2 {
		t.Fatalf("Review() = %d statements, want 2", len(review.Statements))
	}
	for _, one := range review.Statements {
		if script[one.Start:one.End] != one.SQL {
			t.Errorf("command %q does not sit at %d..%d", one.SQL, one.Start, one.End)
		}
	}
}

// The three faults this side can prove, and nothing else. A command that will not parse is one of
// them here and would not be in SQL, because this grammar is small enough to read whole.
func TestReviewReportsWhatItCanProve(t *testing.T) {
	tests := []struct {
		name string
		mql  string
		says string
	}{
		{"unclosed string", `db.users.find({ a: "one })`, "string is never closed"},
		{"unclosed comment", "/* note\ndb.users.find({})", "block comment is never closed"},
		{"unclosed brace", `db.users.find({ a: 1 )`, "bracket"},
		{"stray closer", `db.users.find({ a: 1 }})`, "nothing to close"},
		{"no value", `db.users.find({ a: })`, "expected a value"},
		{"not a command at all", `users.find({})`, "begins with `db`"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			review := text{}.Review(tt.mql)
			if len(review.Problems) == 0 {
				t.Fatalf("Review(%q) reported no problem, want one", tt.mql)
			}

			found := false
			for _, problem := range review.Problems {
				if strings.Contains(problem.Message, tt.says) {
					found = true
				}
				if problem.Start < 0 || problem.End > len(tt.mql) || problem.Start > problem.End {
					t.Errorf("problem sits at %d..%d, outside the script", problem.Start, problem.End)
				}
			}
			if !found {
				t.Errorf("problems = %v, want one saying %q", review.Problems, tt.says)
			}
		})
	}
}

// A command that has only not been finished yet is not a command that is wrong. Review runs on every
// keystroke, so each of these is what a command looks like on the way to being typed — and a squiggle
// that is under the text the whole time it is being written is one the reader stops seeing.
func TestReviewIsQuietAboutACommandBeingTyped(t *testing.T) {
	for _, mql := range []string{
		"db",
		"db.",
		"db.users",
		"db.users.",
		"db.users.find",
	} {
		t.Run(mql, func(t *testing.T) {
			review := text{}.Review(mql)
			if len(review.Problems) != 0 {
				t.Errorf("Review(%q) = %v, want no problem", mql, review.Problems)
			}
		})
	}
}

// What is truncated for real still reports, because a bracket with no partner is the lexer's to prove
// and not the parser's — which is what makes the rule above cost nothing.
func TestReviewStillReportsATruncatedCommand(t *testing.T) {
	review := text{}.Review(`db.users.find({ live: true`)
	if len(review.Problems) == 0 {
		t.Fatalf("Review() reported no problem, want one about the bracket")
	}
}

// A command with nothing wrong with it reports nothing, which is what makes the warnings above worth
// reading at all.
func TestReviewIsSilentAboutAWholeScript(t *testing.T) {
	const script = `// the live ones
db.users.find({ live: true }).limit(10);

db.orders.aggregate([
	{ $match: { total: { $gt: 100 } } },
	{ $group: { _id: "$customer", spend: { $sum: "$total" } } }
]);`

	review := text{}.Review(script)
	if len(review.Problems) != 0 {
		t.Errorf("Problems = %v, want none", review.Problems)
	}
	if len(review.Statements) != 2 {
		t.Errorf("Review() = %d statements, want 2", len(review.Statements))
	}
}

func TestFormatLaysACommandOut(t *testing.T) {
	tests := []struct {
		name string
		mql  string
		want string
	}{
		{
			"short stays on its line",
			`db.users.find({a:1})`,
			`db.users.find({ a: 1 })`,
		},
		{
			"a wide document goes down the page",
			`db.users.find({a:1,b:2,c:3,d:4})`,
			"db.users.find({\n\ta: 1,\n\tb: 2,\n\tc: 3,\n\td: 4\n})",
		},
		{
			"a nested block opens its parent",
			`db.users.updateMany({a:1},{$set:{b:2,c:3,d:4,e:5}})`,
			"db.users.updateMany({ a: 1 }, {\n\t$set: {\n\t\tb: 2,\n\t\tc: 3,\n\t\td: 4,\n\t\te: 5\n\t}\n})",
		},
		{
			"a chain keeps its line",
			`db.users.find({}).sort({name:-1}).limit(10)`,
			`db.users.find({}).sort({ name: -1 }).limit(10)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := text{}.Format(tt.mql)
			if got != tt.want {
				t.Errorf("Format() =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

// Formatting twice has to be the same as formatting once, or a reader who pressed the key again
// would watch their script move.
func TestFormatIsIdempotent(t *testing.T) {
	scripts := []string{
		`db.users.find({a:1,b:2,c:3,d:4})`,
		`db.users.updateMany({a:1},{$set:{b:2,c:3,d:4,e:5}})`,
		`db.orders.aggregate([{$match:{a:1}},{$group:{_id:"$c",n:{$sum:1}}}]);db.users.find({})`,
		"// a note\ndb.users.find({})",
	}

	for _, script := range scripts {
		t.Run(script, func(t *testing.T) {
			once := text{}.Format(script)
			twice := text{}.Format(once)
			if twice != once {
				t.Errorf("Format() twice =\n%s\nwant\n%s", twice, once)
			}
		})
	}
}

// A formatted command still has to *run*, which means it still has to parse into the same command.
func TestFormatChangesNothingTheServerWouldNotice(t *testing.T) {
	const script = `db.users.updateMany({tier:"gold"},{$set:{note:"a;b {c}",at:ISODate("2024-01-31T09:00:00Z")}})`

	before, err := parse(script)
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}
	after, err := parse(text{}.Format(script))
	if err != nil {
		t.Fatalf("parse(Format()) error = %v", err)
	}

	if documentText(before.Filter()) != documentText(after.Filter()) {
		t.Errorf("filter changed: %s became %s",
			documentText(before.Filter()), documentText(after.Filter()))
	}
	if documentText(before.Arg(1)) != documentText(after.Arg(1)) {
		t.Errorf("update changed: %s became %s",
			documentText(before.Arg(1)), documentText(after.Arg(1)))
	}
}

// A script the lexer cannot make sense of comes back exactly as it went in: the one thing worse than
// an unformatted script is a formatted one that no longer runs.
func TestFormatLeavesAnUnterminatedScriptAlone(t *testing.T) {
	const script = `db.users.find({ a: "one })`

	got := text{}.Format(script)
	if got != script {
		t.Errorf("Format() = %q, want the script untouched", got)
	}
}

// Format lays out a *value* and not only a command, because the value viewer shows one field of one
// document and asks this side to lay it out: a bare key, an `ObjectId(…)` and where a line breaks
// are this grammar's, and there is no second formatter on the frontend. See
// docs/features/value-viewer.md.
func TestFormatLaysOutABareValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "a document breaks one field to a line",
			value: `{ id: "d42", status: "completed", at: ISODate("2026-01-01T00:00:00Z"), score: 0.68 }`,
			want: "{\n" +
				"\tid: \"d42\",\n" +
				"\tstatus: \"completed\",\n" +
				"\tat: ISODate(\"2026-01-01T00:00:00Z\"),\n" +
				"\tscore: 0.68\n" +
				"}",
		},
		// The same rule the editor is laid out by: a brace a line carries stays on it, so formatting
		// a small document twice is formatting it once.
		{
			name:  "one a line carries stays on it",
			value: `{ a: 1, b: 2 }`,
			want:  `{ a: 1, b: 2 }`,
		},
		{
			name:  "an array breaks like a document",
			value: `[1, "two", 3, 4]`,
			want:  "[\n\t1,\n\t\"two\",\n\t3,\n\t4\n]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (text{}).Format(tt.value); got != tt.want {
				t.Errorf("Format() =\n%s\n\nwant\n%s", got, tt.want)
			}
		})
	}
}

// A field holding markup is a string holding escaped quotes, and this notation is JavaScript, where a
// backslash escapes one. A lexer that read that quote as the end of the string would cut the value in
// half and lay the rest of it out as code, so the fields after it lose their lines and the value the
// reader is shown is not the value the cell holds. See `sqlkit.Shell`.
func TestFormatKeepsAStringHoldingEscapedQuotes(t *testing.T) {
	const content = `"<div class=\"post\" style=\"margin: 0px\"><p>%user_name%</p></div>"`
	value := `{ id: 53, subject: "Interview for %user_name%", content: ` + content +
		`, attachments: [], order: 2 }`

	got := (text{}).Format(value)

	if !strings.Contains(got, content) {
		t.Errorf("Format() lost the markup:\n%s\n\nthe string should still hold\n%s", got, content)
	}
	// One field to a line, which is the whole point of laying a document out: a token stream broken
	// inside a string mismatches every brace after it and crams the fields onto one line.
	for _, want := range []string{"\tid: 53,", "\tattachments: [],", "\torder: 2"} {
		if !strings.Contains(got, want+"\n") && !strings.HasSuffix(got, want) {
			t.Errorf("Format() =\n%s\n\nwant a line %q", got, want)
		}
	}
}
