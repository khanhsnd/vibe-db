package sqlserver

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// run is what sqlkit.DDL executes its statements through, and the one place a schema change meets
// this adapter's error map. Every statement already names its database in its own three-part table
// name, so nothing has to be selected for it — which is why there is no pinning here and no pool
// per database anywhere in this adapter.
func (s *session) run(ctx context.Context, _, statement string) error {
	if _, err := s.db.ExecContext(ctx, statement); err != nil {
		return translateError(err)
	}
	return nil
}

// A rename here is a *procedure* rather than a clause, and it is the same procedure for a table and
// for a column — which is why both are written out and neither comes from `sqlkit.DDL`.
//
// `sp_rename` takes its arguments as strings, and the object is named by a schema-qualified name
// inside one. The new name goes in bare: naming a schema there would be read as an attempt to move
// the object, which is not what a rename is and which SQL Server refuses.
func (s *session) RenameTable(
	ctx context.Context,
	ns driver.Namespace,
	table, name string,
) error {
	return s.rename(ctx, ns, dialect.Qualify(ns.Schema, table), name, "OBJECT")
}

func (s *session) RenameColumn(
	ctx context.Context,
	ns driver.Namespace,
	table, column, name string,
) error {
	target := dialect.Qualify(ns.Schema, table) + "." + dialect.QuoteIdent(column)
	return s.rename(ctx, ns, target, name, "COLUMN")
}

func (s *session) rename(
	ctx context.Context,
	ns driver.Namespace,
	target, name, kind string,
) error {
	statement := "EXEC " + dialect.QuoteIdent(ns.Database) + ".sys.sp_rename " +
		dialect.QuoteString(target) + ", " + dialect.QuoteString(name) +
		", " + dialect.QuoteString(kind)
	return s.run(ctx, ns.Database, statement)
}

// SetPrimaryKey has to name the constraint it is replacing, which SQL Server keeps in
// sys.key_constraints rather than deriving from the table — the same arrangement PostgreSQL has,
// and for the same reason: a key here is a constraint with a name of its own.
func (s *session) SetPrimaryKey(
	ctx context.Context,
	ns driver.Namespace,
	table string,
	columns []string,
) error {
	existing, err := s.primaryKeyName(ctx, ns, table)
	if err != nil {
		return err
	}

	actions := []string{}
	if existing != "" {
		actions = append(actions, "DROP CONSTRAINT "+dialect.QuoteIdent(existing))
	}
	if len(columns) > 0 {
		actions = append(actions,
			"ADD PRIMARY KEY ("+strings.Join(dialect.QuoteAll(columns), ", ")+")")
	}
	if len(actions) == 0 {
		return nil
	}

	// Unlike every other engine here, SQL Server will not take a drop and an add in one ALTER
	// TABLE: each action is a statement of its own. The order is what matters — the old key has to
	// be gone before the new one can be declared.
	for _, action := range actions {
		statement := "ALTER TABLE " + dialect.Table(ns, table) + " " + action
		if err := s.run(ctx, ns.Database, statement); err != nil {
			return err
		}
	}
	return nil
}

func (s *session) primaryKeyName(
	ctx context.Context,
	ns driver.Namespace,
	table string,
) (string, error) {
	catalogue := dialect.QuoteIdent(ns.Database)

	query := `
		SELECT k.name
		FROM ` + catalogue + `.sys.key_constraints k
		JOIN ` + catalogue + `.sys.tables t ON t.object_id = k.parent_object_id
		JOIN ` + catalogue + `.sys.schemas s ON s.schema_id = t.schema_id
		WHERE s.name = @p1 AND t.name = @p2 AND k.type = 'PK'`

	var name string
	err := s.db.QueryRowContext(ctx, query, ns.Schema, table).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", translateError(err)
	}
	return name, nil
}

// CreateStatements has to *describe* the table, because SQL Server cannot be asked: there is no
// SHOW CREATE TABLE and the script Management Studio writes is written by Management Studio. So
// the shape the tree already reads is spelled back out through the same dialect every other
// statement here goes through — the columns and the key as one CREATE TABLE, then an index each,
// then a constraint each.
//
// The index behind the primary key is left out: it arrives with the table's own PRIMARY KEY clause,
// and declaring it a second time is a statement the server refuses. What a description cannot carry
// is an identity column, a computed column or a filtered index — the honest cost of an engine that
// will not answer for itself.
func (s *session) CreateStatements(
	ctx context.Context,
	ns driver.Namespace,
	table string,
) ([]string, error) {
	structure, err := s.TableStructure(ctx, ns, table)
	if err != nil {
		return nil, err
	}

	spec := sqlkit.TableSpecOf(ns, table, structure.Columns)
	statements := dialect.CreateTableStatements(spec)
	for _, index := range structure.Indexes {
		if index.Primary {
			continue
		}
		// The method the catalogue reported is what the index *is* — clustered or not — and not an
		// access method a CREATE INDEX takes a `USING` for. Dropping it is what keeps the statement
		// one the server will accept.
		declared := sqlkit.IndexSpecOf(index)
		declared.Method = ""
		statements = append(statements, dialect.CreateIndex(ns, table, declared))
	}
	for _, key := range structure.ForeignKeys {
		statements = append(statements,
			dialect.AddForeignKey(ns, table, sqlkit.ForeignKeySpecOf(key)))
	}

	return statements, nil
}

// Source asks the server for its own words, which is the one declaration SQL Server *does* keep:
// `sys.sql_modules` holds the text of the statement that made every view, function, procedure and
// trigger, exactly as it was submitted. That is why this is a read where CreateStatements above is
// a description — a table's shape is derived from a dozen catalogue views, and a module is not.
//
// A trigger needs no branch of its own. It is an object of its table's schema and a module like the
// rest, so the same join finds it under the same two names.
//
// `OBJECT_DEFINITION` is not used. It reads an object id in whatever database the connection is
// pointed at, and this adapter has one pool for the whole server — so the read is qualified the
// way every other statement here is, by naming the database in front of `sys`.
func (s *session) Source(
	ctx context.Context,
	ns driver.Namespace,
	ref driver.SourceRef,
) (driver.Source, error) {
	catalogue := dialect.QuoteIdent(ns.Database)

	// A sequence is not a module: it has no body, only numbers, so it is described rather than read
	// — see `sequenceSource`. The two kinds this engine has not got at all say so.
	switch ref.Kind {
	case driver.SourceSequence:
		return s.sequenceSource(ctx, ns, ref)
	case driver.SourceMaterializedView:
		return driver.Source{}, domain.Unsupported(
			"a materialized view: an indexed view here is a view with an index on it")
	case driver.SourceEvent:
		return driver.Source{}, domain.Unsupported(
			"an event: a SQL Server Agent job is made by calling msdb.dbo.sp_add_job")
	}

	query := `
		SELECT m.definition
		FROM ` + catalogue + `.sys.sql_modules m
		JOIN ` + catalogue + `.sys.objects o ON o.object_id = m.object_id
		JOIN ` + catalogue + `.sys.schemas s ON s.schema_id = o.schema_id
		WHERE s.name = @p1 AND o.name = @p2`

	// The definition is NULL for a module the account may not read, and for an encrypted one. Both
	// are "there is one and you cannot see it", which is not the same answer as "there is none".
	var definition *string
	err := s.db.QueryRowContext(ctx, query, ns.Schema, ref.Name).Scan(&definition)
	if errors.Is(err, sql.ErrNoRows) {
		return driver.Source{}, domain.ErrUnknownDefinition
	}
	if err != nil {
		return driver.Source{}, translateError(err)
	}
	if definition == nil {
		return driver.Source{}, domain.Unsupported("reading this definition: the server keeps it encrypted")
	}

	return driver.Source{SourceRef: ref, Definition: strings.TrimSpace(*definition)}, nil
}

// sequenceSource spells a sequence's declaration from `sys.sequences`, which is a *description* in
// the sense CreateStatements is one: the catalogue keeps the numbers rather than the statement.
//
// `current_value` is where the sequence has got to, and it is what a replayed `START WITH` carries,
// for the reason Oracle's `last_number` is: a sequence made again from its original start would hand
// out numbers rows already hold. Every number is read as text, a sequence here being any integer
// type the reader declared it as — including `decimal(38,0)`, which no Go integer holds.
func (s *session) sequenceSource(
	ctx context.Context,
	ns driver.Namespace,
	ref driver.SourceRef,
) (driver.Source, error) {
	catalogue := dialect.QuoteIdent(ns.Database)

	query := `
		SELECT TYPE_NAME(q.user_type_id),
		       CONVERT(nvarchar(64), q.current_value),
		       CONVERT(nvarchar(64), q.increment),
		       CONVERT(nvarchar(64), q.minimum_value),
		       CONVERT(nvarchar(64), q.maximum_value),
		       CONVERT(nvarchar(64), q.cache_size),
		       q.is_cycling
		FROM ` + catalogue + `.sys.sequences q
		JOIN ` + catalogue + `.sys.schemas s ON s.schema_id = q.schema_id
		WHERE s.name = @p1 AND q.name = @p2`

	var kind, start, increment, minimum, maximum string
	var cache *string
	var cycles bool

	err := s.db.QueryRowContext(ctx, query, ns.Schema, ref.Name).
		Scan(&kind, &start, &increment, &minimum, &maximum, &cache, &cycles)
	if errors.Is(err, sql.ErrNoRows) {
		return driver.Source{}, domain.ErrUnknownDefinition
	}
	if err != nil {
		return driver.Source{}, translateError(err)
	}

	// A NULL cache size is `CACHE` with no number — the server sizing it — where a zero is the
	// reader having asked for none at all. The two are different clauses and neither is the other.
	caching := "CACHE"
	switch {
	case cache == nil:
	case *cache == "0":
		caching = "NO CACHE"
	default:
		caching = "CACHE " + *cache
	}

	cycling := "NO CYCLE"
	if cycles {
		cycling = "CYCLE"
	}

	definition := "CREATE SEQUENCE " + dialect.Table(ns, ref.Name) + "\n" +
		"AS " + kind + "\n" +
		"START WITH " + start + "\n" +
		"INCREMENT BY " + increment + "\n" +
		"MINVALUE " + minimum + "\n" +
		"MAXVALUE " + maximum + "\n" +
		caching + "\n" +
		cycling
	return driver.Source{SourceRef: ref, Definition: definition}, nil
}
