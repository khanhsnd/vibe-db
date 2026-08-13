package sqlkit

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// CallStyle is how an engine runs a stored routine, and how a value the routine hands *back*
// reaches the reader. The two are one flag rather than two because no engine mixes them: an engine
// that spells the call one way receives an OUT parameter that way and no other.
//
//   - CallWithVariables: the call is `CALL p(…)` and an OUT parameter is a **user variable** the
//     connection holds, so the call is three statements — set the variable, call, read it back.
//     MySQL.
//   - CallDirect: the call is `CALL p(…)` and the engine answers with the OUT values as a **row**,
//     so there is nothing to set and nothing to read afterwards. PostgreSQL, and the one style where
//     a function is read in the FROM clause — which is what lets a function with OUT parameters and
//     a function returning a table be the same call.
//   - CallInBatch: the call is `EXEC p …` and an OUT parameter is a variable **declared in the same
//     batch**, so the whole call is one statement holding three. SQL Server.
//   - CallInBlock: the call is a PL/SQL block, which answers with nothing at all. Oracle, and the
//     one engine where an OUT parameter is not reachable without binding a variable — so a routine
//     that has one says so rather than being called with a value it would refuse.
//
// An engine that leaves it empty has no stored routine to call: SQLite, whose functions belong to
// the program that opened the file.
type CallStyle string

const (
	CallNone          CallStyle = ""
	CallWithVariables CallStyle = "variables"
	CallDirect        CallStyle = "direct"
	CallInBatch       CallStyle = "batch"
	CallInBlock       CallStyle = "block"
)

// Call is a routine call spelled: the statements that run it, in order, and whether the last of
// them answers with **rows**. They are run on one connection and the last one is the answer, which
// is why nothing here has to say which statement that is.
//
// No statement at all is an engine that has no grammar for this call — see CallStyle, and see
// DDL.CallRoutine, which is where that becomes `domain.Unsupported` naming which.
type Call struct {
	Statements []string
	Reads      bool
}

// paramModes is every word the five catalogues have for what a routine does with a parameter, in
// one map: `IN` / `OUT` / `INOUT` on two of them, `IN/OUT` on a third, and one letter each on
// PostgreSQL's, whose `v` is a variadic — a value taken, however many of them — and whose `t` is a
// column of a returned table, which is a value handed back.
//
// One map rather than one per adapter because none of the words collide, and a mode read wrongly
// would decide whether a value is written into the call or read out of it.
var paramModes = map[string]driver.ParamMode{
	"in":     driver.ParamIn,
	"i":      driver.ParamIn,
	"v":      driver.ParamIn,
	"out":    driver.ParamOut,
	"o":      driver.ParamOut,
	"t":      driver.ParamOut,
	"inout":  driver.ParamInOut,
	"in/out": driver.ParamInOut,
	"b":      driver.ParamInOut,
}

// ParamModeOf reads an engine's own word for a parameter's mode. A word none of them uses is a
// value **taken**, which is what a parameter with nothing said about it is on every engine here.
func ParamModeOf(word string) driver.ParamMode {
	if mode, known := paramModes[strings.ToLower(strings.TrimSpace(word))]; known {
		return mode
	}
	return driver.ParamIn
}

// ScanRoutineParams folds a three-column catalogue read — a parameter's name, this engine's word
// for what the routine does with it, and its type — into the list a call is filled in from. It is
// here rather than in each adapter because what differs between the engines is the *read*, and the
// fold does not differ at all.
//
// A parameter with no name is named by its position: PostgreSQL will declare a function's arguments
// without naming them, and a box the reader fills in has to be labelled something.
func ScanRoutineParams(rows *sql.Rows) ([]driver.RoutineParam, error) {
	defer rows.Close()

	params := []driver.RoutineParam{}
	for rows.Next() {
		var name, mode, kind *string
		if err := rows.Scan(&name, &mode, &kind); err != nil {
			return nil, err
		}

		found := driver.RoutineParam{
			Name: strings.TrimSpace(textOf(name)),
			Mode: ParamModeOf(textOf(mode)),
			Type: strings.TrimSpace(textOf(kind)),
		}
		if found.Name == "" {
			found.Name = "arg" + strconv.Itoa(len(params)+1)
		}
		params = append(params, found)
	}
	return params, rows.Err()
}

// callVariable is the variable an OUT parameter's value is handed back in, named by its position
// rather than by the parameter, so a parameter called `select` or `x y` is still a name nothing has
// to quote. `@name` happens to be both a MySQL user variable and a T-SQL local one, which is why
// the two styles that need one need no flag to spell it.
func callVariable(at int) string {
	return "@vibe_arg_" + strconv.Itoa(at+1)
}

// Call spells running one routine. A function is *read* — it answers with a value, and a value is
// something a SELECT has — and a procedure is *run*, which is a different statement on every one of
// these engines and the whole reason CallStyle exists.
func (d Dialect) Call(ns driver.Namespace, call driver.RoutineCall) Call {
	if d.Calls == CallNone {
		return Call{}
	}
	if call.Kind == driver.SourceFunction {
		return d.callFunction(ns, call)
	}
	return d.callProcedure(ns, call)
}

// callFunction reads a function's value. Every argument is written in and nothing is handed back
// besides the answer, so an OUT parameter is a shape only one of these engines can read at all:
// PostgreSQL puts a function in the FROM clause, where its OUT parameters *are* the columns, and
// every other engine would need a variable a SELECT has nowhere to put.
func (d Dialect) callFunction(ns driver.Namespace, call driver.RoutineCall) Call {
	name := d.Table(ns, call.Name)

	values := make([]string, 0, len(call.Args))
	for _, arg := range call.Args {
		if arg.Mode != driver.ParamIn && d.Calls != CallDirect {
			return Call{}
		}
		// An OUT parameter is not passed at all where the engine answers with it as a column, which
		// is the one place a function's argument list is shorter than its parameter list.
		if arg.Mode == driver.ParamOut {
			continue
		}
		values = append(values, d.CallArgument(arg))
	}

	read := "SELECT " + name + "(" + strings.Join(values, ", ") + ")" + d.From()
	if d.Calls == CallDirect {
		read = "SELECT * FROM " + name + "(" + strings.Join(values, ", ") + ")"
	}
	return Call{Statements: []string{read}, Reads: true}
}

// callProcedure runs a procedure, in the four shapes CallStyle names. What differs between them is
// where a value the procedure hands back is put: nowhere, in a variable of the connection, in a
// variable of the batch, or in the row the engine answers the call with.
func (d Dialect) callProcedure(ns driver.Namespace, call driver.RoutineCall) Call {
	name := d.Table(ns, call.Name)

	switch d.Calls {
	case CallDirect:
		// Every parameter is written in, an OUT one as NULL, and the engine answers with a row
		// holding what it made of them.
		return Call{
			Statements: []string{"CALL " + name + "(" + strings.Join(d.arguments(call), ", ") + ")"},
			Reads:      true,
		}

	case CallWithVariables:
		return d.callThroughVariables(name, call)

	case CallInBatch:
		return d.callInOneBatch(name, call)

	case CallInBlock:
		// A block is a statement and answers with nothing, so a parameter this engine would hand
		// back has nowhere to go: it is bound, and a bound call is not the statement a preview
		// showed. The whole call spells nothing rather than one that would be refused.
		for _, arg := range call.Args {
			if arg.Mode != driver.ParamIn {
				return Call{}
			}
		}
		return Call{
			Statements: []string{
				"BEGIN " + name + "(" + strings.Join(d.arguments(call), ", ") + "); END;",
			},
		}

	default:
		return Call{}
	}
}

// callThroughVariables is the call on an engine whose OUT parameter is a **user variable**: it is
// set, passed, and read back — three statements on one connection, because a user variable belongs
// to the connection and nothing else would find it again.
//
// An OUT parameter is set as well as read, so the variable exists whatever the routine does with
// it: a routine that returns early would otherwise hand back whatever the connection last left
// there.
func (d Dialect) callThroughVariables(name string, call driver.RoutineCall) Call {
	before := []string{}
	values := make([]string, 0, len(call.Args))
	given := []string{}

	for at, arg := range call.Args {
		if arg.Mode == driver.ParamIn {
			values = append(values, d.CallArgument(arg))
			continue
		}
		variable := callVariable(at)
		before = append(before, "SET "+variable+" = "+d.CallArgument(arg))
		values = append(values, variable)
		given = append(given, variable+" AS "+d.QuoteIdent(arg.Name))
	}

	statements := append(before, "CALL "+name+"("+strings.Join(values, ", ")+")")
	if len(given) > 0 {
		statements = append(statements, "SELECT "+strings.Join(given, ", "))
	}
	return Call{Statements: statements, Reads: true}
}

// callInOneBatch is the call on an engine whose variable belongs to the **batch** rather than to
// the connection: the declaration, the call and the read are one statement, since a second batch
// would not know the name. It is the one statement in this package that holds more than one, and it
// is honest — a batch is what the server is sent and what a preview therefore has to show.
func (d Dialect) callInOneBatch(name string, call driver.RoutineCall) Call {
	statements := []string{}
	values := make([]string, 0, len(call.Args))
	given := []string{}

	for at, arg := range call.Args {
		if arg.Mode == driver.ParamIn {
			values = append(values, d.CallArgument(arg))
			continue
		}
		variable := callVariable(at)
		// The type is the catalogue's own word for it and travels verbatim, the way a column's
		// does: a variable receiving a value has to be declared as what that value is.
		statements = append(statements, "DECLARE "+variable+" "+strings.TrimSpace(arg.Type))
		if arg.Mode == driver.ParamInOut {
			statements = append(statements, "SET "+variable+" = "+d.CallArgument(arg))
		}
		values = append(values, variable+" OUTPUT")
		given = append(given, variable+" AS "+d.QuoteIdent(arg.Name))
	}

	execute := "EXEC " + name
	if len(values) > 0 {
		execute += " " + strings.Join(values, ", ")
	}

	statements = append(statements, execute)
	if len(given) > 0 {
		statements = append(statements, "SELECT "+strings.Join(given, ", "))
	}
	return Call{Statements: []string{strings.Join(statements, ";\n")}, Reads: true}
}

// arguments is every parameter written in, in order, which is what an engine that takes an OUT
// parameter's place in the list needs.
func (d Dialect) arguments(call driver.RoutineCall) []string {
	values := make([]string, 0, len(call.Args))
	for _, arg := range call.Args {
		values = append(values, d.CallArgument(arg))
	}
	return values
}

// CallArgument is one value on its way into a call: a string literal, or the keyword for a value
// that is not there. It is quoted as a literal for the reason a filter's value is — every one of
// these engines reads one back into a parameter of any type — and nothing is bound, because a call
// that bound its arguments could not be shown as the statement it runs.
func (d Dialect) CallArgument(arg driver.RoutineArg) string {
	if arg.Value == nil {
		return "NULL"
	}
	return d.QuoteString(*arg.Value)
}

// CallStatements spells running a routine and runs none of it, which is what the reader is shown
// before anything happens — the arrangement AlterStatements and SourceStatements have.
func (x DDL) CallStatements(ns driver.Namespace, call driver.RoutineCall) []string {
	return x.Dialect.Call(ns, call).Statements
}

// CallRoutine runs exactly what CallStatements showed, in order, on one connection put inside the
// routine's namespace — the connection a definition is saved on, and inside the reader's manual
// transaction where one is open.
//
// One connection is not a convenience here: on the engine whose OUT parameter is a user variable,
// a second connection would read a variable nothing had set.
func (x DDL) CallRoutine(
	ctx context.Context,
	ns driver.Namespace,
	call driver.RoutineCall,
) (driver.QueryResult, error) {
	spelled := x.Dialect.Call(ns, call)
	// An adapter with nothing to pin has one connection's worth of namespace and no way to hold a
	// variable between two statements, which is the whole of what this needs — so it is the same
	// answer as having no grammar for the call at all.
	if len(spelled.Statements) == 0 || x.Pin == nil {
		return driver.QueryResult{}, domain.Unsupported(x.callMissing(call))
	}

	conn, release, err := x.Pin(ctx, ns)
	if err != nil {
		return driver.QueryResult{}, err
	}
	defer release()

	answer := driver.QueryResult{Rows: [][]*string{}}
	last := len(spelled.Statements) - 1

	for at, statement := range spelled.Statements {
		if at == last && spelled.Reads {
			read, err := Read(ctx, conn, statement, call.MaxRows, nil)
			if err != nil {
				return driver.QueryResult{}, x.Translate(err)
			}
			return read, nil
		}

		ran, err := Exec(ctx, conn, statement)
		if err != nil {
			return driver.QueryResult{}, x.Translate(err)
		}
		if at == last {
			answer = ran
		}
	}
	return answer, nil
}

// callMissing is what this engine has not got, named. There are two ways to spell nothing and they
// are two different answers: an engine with no stored routines at all, and one that has them but
// hands a value back only to a bound variable.
func (x DDL) callMissing(call driver.RoutineCall) string {
	if x.Dialect.Calls == CallNone {
		return "calling a routine"
	}
	for _, arg := range call.Args {
		if arg.Mode != driver.ParamIn {
			return "an OUT parameter: this engine hands one back only to a bound variable"
		}
	}
	return "calling a routine"
}
