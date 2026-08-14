package session

import (
	"errors"
	"reflect"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// writes is every method of `driver.Session` a read-only connection holds back. It is the
// classification itself rather than a copy of one: the test below walks the interface and asks of
// **every** method whether it is refused, so a method added to `driver.Session` and left out of this
// map fails here until somebody has decided which it is.
//
// Everything absent is deliberately allowed — a read, a speller that runs nothing, or a transaction
// that could not write from inside one anyway. See the note on readOnly.
var writes = map[string]bool{
	"UpdateCell":              true,
	"InsertRow":               true,
	"DeleteRow":               true,
	"Query":                   true,
	"CreateDatabase":          true,
	"DropDatabase":            true,
	"CreateTable":             true,
	"RenameTable":             true,
	"DuplicateTable":          true,
	"DropTable":               true,
	"TruncateTable":           true,
	"EmptyTable":              true,
	"AddColumn":               true,
	"AlterColumn":             true,
	"RenameColumn":            true,
	"DropColumn":              true,
	"SetPrimaryKey":           true,
	"CreateIndex":             true,
	"DropIndex":               true,
	"CreateForeignKey":        true,
	"DropForeignKey":          true,
	"AddPartition":            true,
	"DropPartition":           true,
	"TruncatePartition":       true,
	"RefreshMaterializedView": true,
	"DropSource":              true,
	"SaveSource":              true,
	"CallRoutine":             true,
	"ApplyAccountChanges":     true,
	"KillProcesses":           true,
	"ApplyProfileChanges":     true,
}

// TestReadOnlyClassifiesEveryMethod is the answer to what embedding costs: a method the wrapper does
// not override is carried through silently, so this asks the *interface* rather than the wrapper.
//
// It calls each method on a wrapper holding **no session at all**. A refusal answers without ever
// reaching the one that is not there; anything else panics on it, which is exactly the difference
// being measured.
func TestReadOnlyClassifiesEveryMethod(t *testing.T) {
	guarded := reflect.ValueOf(readOnly{})
	session := reflect.TypeFor[driver.Session]()

	for i := range session.NumMethod() {
		name := session.Method(i).Name

		if refuses(guarded, name) != writes[name] {
			t.Errorf("%s: refused = %v, want %v — classify it in `writes` and, if it writes, "+
				"override it in readonly.go", name, !writes[name], writes[name])
		}
	}

	for name := range writes {
		if _, ok := session.MethodByName(name); !ok {
			t.Errorf("%s: no longer a method of driver.Session", name)
		}
	}
}

// refuses calls one method with zero-valued arguments and says whether it answered with
// domain.ErrReadOnly. Reaching the absent session panics, which is recovered and read as "not
// refused" — the one thing a guard must never do.
func refuses(guarded reflect.Value, name string) (refused bool) {
	defer func() {
		if recover() != nil {
			refused = false
		}
	}()

	method := guarded.MethodByName(name)
	args := make([]reflect.Value, method.Type().NumIn())
	for i := range args {
		args[i] = reflect.Zero(method.Type().In(i))
	}

	for _, answer := range method.Call(args) {
		if err, ok := answer.Interface().(error); ok && errors.Is(err, domain.ErrReadOnly) {
			return true
		}
	}
	return false
}

// TestReadOnlyReadsTheScript is the other half of the guard: a typed statement is neither plainly a
// read nor plainly a write, and what decides it is the engine's own reviewer. A real dialect is used
// rather than a stub, because half of what is being tested is what that reviewer actually answers.
func TestReadOnlyReadsTheScript(t *testing.T) {
	guarded := readOnly{text: sqlkit.MySQL}

	cases := []struct {
		script  string
		refused bool
	}{
		{"SELECT * FROM users", false},
		{"select id from users; select name from users", false},
		{"DELETE FROM users WHERE id = 1", true},
		{"SELECT 1; DROP TABLE users", true},
		// A CTE writes without saying so in its first word, and a refusal defaults to refusing
		// where a confirmation defaults to allowing — see domain.Access.
		{"WITH gone AS (DELETE FROM users RETURNING *) SELECT * FROM gone", true},
	}

	for _, one := range cases {
		err := guarded.reviewed(one.script)
		if errors.Is(err, domain.ErrReadOnly) != one.refused {
			t.Errorf("%q: refused = %v, want %v", one.script, err != nil, one.refused)
		}
	}
}

// TestReadOnlyWithNoReviewer is the guard being asked to guard something it cannot read. It refuses,
// because the alternative is running it.
func TestReadOnlyWithNoReviewer(t *testing.T) {
	if err := (readOnly{}).reviewed("SELECT 1"); !errors.Is(err, domain.ErrReadOnly) {
		t.Errorf("a session with no reviewer ran a statement: %v", err)
	}
}

// TestReadOnlyNamesWhatItRefused is what the reader is actually told. A refusal that said only
// "refused" would leave them looking for which of six statements it was.
func TestReadOnlyNamesWhatItRefused(t *testing.T) {
	var refusal domain.ReadOnlyError

	err := readOnly{text: sqlkit.MySQL}.reviewed("SELECT 1;\nTRUNCATE TABLE users")
	if !errors.As(err, &refusal) || refusal.What != "TRUNCATE" {
		t.Fatalf("refused %q, want TRUNCATE (%v)", refusal.What, err)
	}
}
