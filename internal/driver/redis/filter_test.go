package redis

import (
	"testing"

	"github.com/khanhsnd/vibe-db/internal/driver"
)

func keyFilter(op driver.TableFilterOp, values ...string) driver.TableFilter {
	return driver.TableFilter{Column: columnKey, Op: op, Values: values}
}

func andGroup(children ...driver.TableCriterion) driver.TableGroup {
	return driver.TableGroup{Conjunction: driver.ConjunctionAnd, Children: children}
}

func TestGlobIsTakenOnlyFromWhatNarrowsTheRead(t *testing.T) {
	tests := []struct {
		name   string
		filter driver.TableGroup
		want   string
	}{
		{"nothing", driver.TableGroup{}, "*"},
		{"begins", andGroup(keyFilter(driver.FilterBegins, "user:")), "user:*"},
		{"contains", andGroup(keyFilter(driver.FilterContains, "session")), "*session*"},
		{"ends", andGroup(keyFilter(driver.FilterEnds, ":lock")), "*:lock"},
		{"equals", andGroup(keyFilter(driver.FilterEqual, "user:1")), "user:1"},
		{
			"a pattern the reader typed stands for itself",
			andGroup(keyFilter(driver.FilterBegins, "user[1]")),
			`user\[1\]*`,
		},
		{
			// The one operator that does not escape: the reader is spelling the glob itself.
			"a glob the reader spelled",
			andGroup(keyFilter(driver.FilterMatches, "*email*")),
			"*email*",
		},
		{
			// Redis has no case-insensitive matching, so the case goes into the pattern and the
			// server does the narrowing all the same.
			"a contains that ignores case",
			andGroup(keyFilter(driver.FilterContainsFold, "User")),
			"*[uU][sS][eE][rR]*",
		},
		{
			// A filter on another column narrows nothing the server can do.
			"a type filter",
			andGroup(driver.TableFilter{Column: columnType, Op: driver.FilterEqual, Values: []string{"hash"}}),
			"*",
		},
		{
			// The rule that matters: a condition inside an OR narrows nothing on its own, so taking
			// a glob from one would hide rows the reader asked for.
			"an alternative",
			driver.TableGroup{
				Conjunction: driver.ConjunctionOr,
				Children: []driver.TableCriterion{
					keyFilter(driver.FilterBegins, "user:"),
					driver.TableFilter{Column: columnType, Op: driver.FilterEqual, Values: []string{"hash"}},
				},
			},
			"*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := glob(tt.filter); got != tt.want {
				t.Errorf("glob() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWantsSaysHowMuchOfEachKeyHasToBeRead(t *testing.T) {
	byName, err := wants(andGroup(keyFilter(driver.FilterBegins, "user:")), nil)
	if err != nil {
		t.Fatalf("wants() error = %v", err)
	}
	if byName.beyondName() {
		t.Error("a filter on the key alone wants more than the name")
	}

	byType, err := wants(
		andGroup(driver.TableFilter{Column: columnType, Op: driver.FilterEqual, Values: []string{"hash"}}),
		nil,
	)
	if err != nil {
		t.Fatalf("wants() error = %v", err)
	}
	if !byType.beyondName() {
		t.Error("a filter on the type wants only the name")
	}

	bySort, err := wants(driver.TableGroup{}, []driver.TableSort{{Column: columnTTL}})
	if err != nil {
		t.Fatalf("wants() error = %v", err)
	}
	if !bySort.beyondName() {
		t.Error("a sort on the TTL wants only the name")
	}
}

func TestWantsRefusesWhatThisSideDoesNotKnow(t *testing.T) {
	if _, err := wants(andGroup(driver.TableFilter{Column: "colour", Op: driver.FilterEqual, Values: []string{"red"}}), nil); err == nil {
		t.Error("a column a key space has not got was accepted")
	}
	if _, err := wants(andGroup(keyFilter("sounds-like", "user")), nil); err == nil {
		t.Error("an operator this app does not know was accepted")
	}
	if _, err := wants(andGroup(keyFilter(driver.FilterBetween, "a")), nil); err == nil {
		t.Error("half a BETWEEN was accepted")
	}
}

func TestMatchesAppliesTheWholeTree(t *testing.T) {
	row := keyRow{name: "user:1", kind: typeHash, ttl: 60_000, size: 120, value: `{"a":"b"}`, valued: true}

	tests := []struct {
		name   string
		filter driver.TableGroup
		want   bool
	}{
		{"nothing narrows nothing", driver.TableGroup{}, true},
		{"a prefix", andGroup(keyFilter(driver.FilterBegins, "user:")), true},
		{"the wrong prefix", andGroup(keyFilter(driver.FilterBegins, "order:")), false},
		{
			"a type",
			andGroup(driver.TableFilter{Column: columnType, Op: driver.FilterEqual, Values: []string{typeHash}}),
			true,
		},
		{
			// The TTL is a number and is compared as one: as text, 60 would come before 9.
			"a clock, numerically",
			andGroup(driver.TableFilter{Column: columnTTL, Op: driver.FilterGreater, Values: []string{"9"}}),
			true,
		},
		{
			"a value",
			andGroup(driver.TableFilter{Column: columnValue, Op: driver.FilterContains, Values: []string{`"a"`}}),
			true,
		},
		{
			"both of two",
			andGroup(
				keyFilter(driver.FilterBegins, "user:"),
				driver.TableFilter{Column: columnType, Op: driver.FilterEqual, Values: []string{typeHash}},
			),
			true,
		},
		{
			"either of two",
			driver.TableGroup{
				Conjunction: driver.ConjunctionOr,
				Children: []driver.TableCriterion{
					keyFilter(driver.FilterBegins, "order:"),
					driver.TableFilter{Column: columnType, Op: driver.FilterEqual, Values: []string{typeHash}},
				},
			},
			true,
		},
		{
			"neither of two",
			driver.TableGroup{
				Conjunction: driver.ConjunctionOr,
				Children: []driver.TableCriterion{
					keyFilter(driver.FilterBegins, "order:"),
					driver.TableFilter{Column: columnType, Op: driver.FilterEqual, Values: []string{typeList}},
				},
			},
			false,
		},
		{"a glob", andGroup(keyFilter(driver.FilterMatches, "user:*")), true},
		{"a glob that misses", andGroup(keyFilter(driver.FilterMatches, "order:*")), false},
		{"a contains that ignores case", andGroup(keyFilter(driver.FilterContainsFold, "USER")), true},
		{"a list", andGroup(keyFilter(driver.FilterInList, "user:1", "user:2")), true},
		{"not in a list", andGroup(keyFilter(driver.FilterNotInList, "user:2")), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := matches(row, tt.filter)
			if err != nil {
				t.Fatalf("matches() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

// A key with no expiry is not a key whose expiry is small, which is the rule every engine has about
// a NULL and the one a clock most invites getting wrong.
func TestAKeyWithNoExpirySatisfiesOnlyIsNull(t *testing.T) {
	forever := keyRow{name: "a", kind: typeString, ttl: noExpiry, size: noSize}

	cases := map[driver.TableFilterOp]bool{
		driver.FilterNull:    true,
		driver.FilterNotNull: false,
		driver.FilterLess:    false,
		driver.FilterGreater: false,
	}

	for op, want := range cases {
		got, err := matchesFilter(forever, driver.TableFilter{
			Column: columnTTL,
			Op:     op,
			Values: []string{"100"},
		})
		if err != nil {
			t.Fatalf("matchesFilter(%q) error = %v", op, err)
		}
		if got != want {
			t.Errorf("matchesFilter(%q) = %v, want %v", op, got, want)
		}
	}
}

func TestOrderSortsByTheColumnsAskedForWithAbsencesLast(t *testing.T) {
	rows := []keyRow{
		{name: "c", ttl: 10_000, size: noSize},
		{name: "a", ttl: noExpiry, size: noSize},
		{name: "b", ttl: 5_000, size: noSize},
	}

	if err := order(rows, []driver.TableSort{{Column: columnTTL}}); err != nil {
		t.Fatalf("order() error = %v", err)
	}
	if rows[0].name != "b" || rows[1].name != "c" || rows[2].name != "a" {
		t.Errorf("order by TTL = %q %q %q, want b c a", rows[0].name, rows[1].name, rows[2].name)
	}

	if err := order(rows, []driver.TableSort{{Column: columnKey, Descending: true}}); err != nil {
		t.Fatalf("order() error = %v", err)
	}
	if rows[0].name != "c" || rows[2].name != "a" {
		t.Errorf("order by key descending = %q … %q, want c … a", rows[0].name, rows[2].name)
	}
}
