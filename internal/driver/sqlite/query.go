package sqlite

import (
	"context"
	"database/sql/driver"
	"regexp"
	"strconv"
	"sync"

	"github.com/khanhsnd/vibe-db/internal/domain"
	dbdriver "github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
	"modernc.org/sqlite"
)

// rowReturning lists the statements SQLite answers with a result set. Anything else goes through
// Exec so the affected-row count survives. `pragma` is among them because a pragma that reads
// answers with rows and one that writes answers with none, and Exec would lose the first.
var rowReturning = map[string]bool{
	"explain": true,
	"pragma":  true,
	"select":  true,
	"values":  true,
	"with":    true,
}

func (s *session) Query(
	ctx context.Context,
	query dbdriver.Query,
) (dbdriver.QueryResult, error) {
	statement := sqlkit.TrimStatement(query.SQL)
	if statement == "" {
		return dbdriver.QueryResult{}, domain.ErrEmptyStatement
	}

	// A plan is read **before** the statement runs, for the reason it is on every other engine:
	// what `DELETE FROM t` was going to do is not what the planner says once the rows are gone.
	var plan *dbdriver.Plan
	if query.Explain {
		plan = explainPlan(ctx, s.db, statement)
	}

	result, err := s.execute(ctx, query, statement)
	if err != nil {
		return dbdriver.QueryResult{}, err
	}

	result.Plan = plan
	return result, nil
}

func (s *session) execute(
	ctx context.Context,
	query dbdriver.Query,
	statement string,
) (dbdriver.QueryResult, error) {
	if !returnsRows(statement) {
		result, err := sqlkit.Exec(ctx, s.db, statement)
		if err != nil {
			return dbdriver.QueryResult{}, translateError(err)
		}
		return result, nil
	}

	result, err := sqlkit.Read(ctx, s.db, statement, query.MaxRows, nil)
	if err != nil {
		return dbdriver.QueryResult{}, translateError(err)
	}

	s.attachSource(ctx, query.Namespace, statement, &result)
	return result, nil
}

// returnsRows decides which of the two paths a statement takes. An INSERT, UPDATE or DELETE
// answers with rows only when it carries a RETURNING clause, which SQLite has had since 3.35 —
// so the leading keyword alone cannot settle it, exactly as on PostgreSQL.
func returnsRows(statement string) bool {
	return rowReturning[sqlkit.LeadingKeyword(statement)] ||
		dialect.HasClause(statement, "returning")
}

// attachSource marks a result writable. The grid needs the table to address a row and the primary
// key to identify it, and the wire format carries neither. Whatever cannot be proven stays
// read-only.
//
// The namespace is always `main` here whatever the statement said, there being only one.
func (s *session) attachSource(
	ctx context.Context,
	ns dbdriver.Namespace,
	statement string,
	result *dbdriver.QueryResult,
) {
	found, ok := dialect.ParseSource(statement)
	if !ok {
		return
	}

	stored, err := s.columns(ctx, ns, found.Table)
	if err != nil {
		return
	}

	keys := sqlkit.PrimaryKeyNames(stored)
	if len(keys) == 0 || !sqlkit.MergeSchema(result.Columns, stored, keys) {
		return
	}

	result.Namespace, result.Table = ns, found.Table
}

// registered makes sure the `regexp` function is added to the driver once. It is registered
// globally rather than per connection because that is the only shape modernc.org/sqlite offers,
// and registering the same name twice is an error rather than a no-op.
var registered sync.Once

// registerRegexp gives SQLite the one operator its grammar has always had and its library has
// never implemented: `x REGEXP y` is defined to call a function named `regexp`, and a build with
// no such function simply refuses the operator.
//
// What is registered is **Go's own** regular expressions, which is worth more than filling a gap.
// A Find in Schema matches a *name* here — in `domain.Matcher`, compiled by the same package — so
// on this engine alone the rows and the names of a search are matched by literally one matcher.
// That is also why the whole of the search is spelled as an expression here: see
// `sqlkit.Dialect.MatchesByRegex`, and note that SQLite's own LIKE ignores case for ASCII and for
// nothing else, which is an answer to the Case Sensitive box that would be true of `a` and false
// of `á`.
//
// The arguments arrive in the operator's order — the *pattern* first — which is what `x REGEXP y`
// means and the opposite of how it reads.
func registerRegexp() {
	registered.Do(func() {
		sqlite.MustRegisterDeterministicScalarFunction("regexp", 2,
			func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
				pattern, ok := args[0].(string)
				if !ok {
					return nil, domain.ValidationError{
						Field:   "text",
						Message: "is not a regular expression this app can read",
					}
				}

				compiled, err := regexp.Compile(pattern)
				if err != nil {
					return nil, domain.ValidationError{
						Field:   "text",
						Message: "is not a regular expression this app can read",
					}
				}

				// A value of any type is matched as the text the grid would have drawn, which is the
				// same rule every other engine's search follows: a search for `36` finds the number
				// as readily as the name. A NULL matches nothing and is not an error.
				return compiled.MatchString(asText(args[1])), nil
			})
	})
}

// asText reads a stored value as the text a match is made against. SQLite hands back whichever of
// its five storage classes the value actually is, so the conversion that every other engine writes
// as a CAST is written here as well — a caller reaching this function directly gets the same answer
// the clause's own cast would have given it.
func asText(value driver.Value) string {
	switch value := value.(type) {
	case string:
		return value
	case []byte:
		return string(value)
	case int64:
		return strconv.FormatInt(value, 10)
	case float64:
		return strconv.FormatFloat(value, 'g', -1, 64)
	case bool:
		return strconv.FormatBool(value)
	default:
		return ""
	}
}
