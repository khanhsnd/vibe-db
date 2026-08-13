package app

import (
	"context"
	"strings"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// schemaTimeout is longer than a read's, because a schema change is not always instant: a
// truncate reclaims storage and an altered column may be rewritten row by row.
const schemaTimeout = 30 * time.Second

func (a *ConnectionAPI) LoadTableStructure(
	sessionID string,
	ns Namespace,
	table string,
) (TableStructure, error) {
	schema, err := a.sessions.Schema(sessionID)
	if err != nil {
		return TableStructure{}, wrapError(err)
	}

	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	structure, err := schema.TableStructure(ctx, ns.toDriver(), table)
	if err != nil {
		return TableStructure{}, wrapError(err)
	}
	return toTableStructureDTO(structure), nil
}

// LoadDatabaseOptions reads the lists the New Database dialog offers — roles, templates,
// encodings, tablespaces, extensions — for whichever of them this engine has.
func (a *ConnectionAPI) LoadDatabaseOptions(sessionID string) (DatabaseOptions, error) {
	schema, err := a.sessions.Schema(sessionID)
	if err != nil {
		return DatabaseOptions{}, wrapError(err)
	}

	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	options, err := schema.DatabaseOptions(ctx)
	if err != nil {
		return DatabaseOptions{}, wrapError(err)
	}
	return toDatabaseOptionsDTO(options), nil
}

// PreviewCreateDatabase is what CreateDatabase would run, so the dialog's SQL Preview shows
// the statement rather than a guess at it. A database with no name yet previews nothing.
func (a *ConnectionAPI) PreviewCreateDatabase(
	sessionID string,
	spec DatabaseSpecInput,
) ([]string, error) {
	schema, err := a.sessions.Schema(sessionID)
	if err != nil {
		return nil, wrapError(err)
	}
	if domain.RequireName("database", spec.Name) != nil {
		return []string{}, nil
	}

	planned := schema.CreateDatabaseSQL(spec.toDriver())
	statements := make([]string, 0, len(planned))
	for _, statement := range planned {
		statements = append(statements, statement.SQL)
	}
	return statements, nil
}

func (a *ConnectionAPI) CreateDatabase(sessionID string, spec DatabaseSpecInput) error {
	if err := domain.RequireName("database", spec.Name); err != nil {
		return wrapError(err)
	}

	return a.changeSchema(sessionID, func(ctx context.Context, schema driver.Schema) error {
		return schema.CreateDatabase(ctx, spec.toDriver())
	})
}

func (a *ConnectionAPI) DropDatabase(sessionID, name string) error {
	if err := domain.RequireName("database", name); err != nil {
		return wrapError(err)
	}

	return a.changeSchema(sessionID, func(ctx context.Context, schema driver.Schema) error {
		return schema.DropDatabase(ctx, name)
	})
}

func (a *ConnectionAPI) CreateTable(sessionID string, spec TableSpecInput) error {
	if err := domain.RequireName("table", spec.Table); err != nil {
		return wrapError(err)
	}
	for _, column := range spec.Columns {
		if err := requireColumn(column); err != nil {
			return wrapError(err)
		}
	}
	if len(spec.Columns) == 0 {
		return wrapError(domain.NameError{What: "column"})
	}

	return a.changeSchema(sessionID, func(ctx context.Context, schema driver.Schema) error {
		return schema.CreateTable(ctx, spec.toDriver())
	})
}

func (a *ConnectionAPI) RenameTable(sessionID string, ns Namespace, table, name string) error {
	if err := domain.RequireNames("table", table, "new table", name); err != nil {
		return wrapError(err)
	}

	return a.changeSchema(sessionID, func(ctx context.Context, schema driver.Schema) error {
		return schema.RenameTable(ctx, ns.toDriver(), table, name)
	})
}

// DuplicateTable copies a table under a new name, with its rows or with its shape alone.
func (a *ConnectionAPI) DuplicateTable(
	sessionID string,
	ns Namespace,
	table, name string,
	withRows bool,
) error {
	if err := domain.RequireNames("table", table, "new table", name); err != nil {
		return wrapError(err)
	}

	return a.changeSchema(sessionID, func(ctx context.Context, schema driver.Schema) error {
		return schema.DuplicateTable(ctx, ns.toDriver(), table, name, withRows)
	})
}

func (a *ConnectionAPI) DropTable(sessionID string, ns Namespace, table string) error {
	if err := domain.RequireName("table", table); err != nil {
		return wrapError(err)
	}

	return a.changeSchema(sessionID, func(ctx context.Context, schema driver.Schema) error {
		return schema.DropTable(ctx, ns.toDriver(), table)
	})
}

func (a *ConnectionAPI) TruncateTable(sessionID string, ns Namespace, table string) error {
	if err := domain.RequireName("table", table); err != nil {
		return wrapError(err)
	}

	return a.changeSchema(sessionID, func(ctx context.Context, schema driver.Schema) error {
		return schema.TruncateTable(ctx, ns.toDriver(), table)
	})
}

func (a *ConnectionAPI) EmptyTable(sessionID string, ns Namespace, table string) error {
	if err := domain.RequireName("table", table); err != nil {
		return wrapError(err)
	}

	return a.changeSchema(sessionID, func(ctx context.Context, schema driver.Schema) error {
		return schema.EmptyTable(ctx, ns.toDriver(), table)
	})
}

func (a *ConnectionAPI) AddColumn(
	sessionID string,
	ns Namespace,
	table string,
	column ColumnInput,
) error {
	if err := domain.RequireName("table", table); err != nil {
		return wrapError(err)
	}
	if err := requireColumn(column); err != nil {
		return wrapError(err)
	}

	return a.changeSchema(sessionID, func(ctx context.Context, schema driver.Schema) error {
		return schema.AddColumn(ctx, ns.toDriver(), table, column.toDriver())
	})
}

func (a *ConnectionAPI) AlterColumn(
	sessionID string,
	ns Namespace,
	table, column string,
	spec ColumnInput,
) error {
	if err := domain.RequireNames("table", table, "column", column); err != nil {
		return wrapError(err)
	}
	if err := requireColumn(spec); err != nil {
		return wrapError(err)
	}

	return a.changeSchema(sessionID, func(ctx context.Context, schema driver.Schema) error {
		return schema.AlterColumn(ctx, ns.toDriver(), table, column, spec.toDriver())
	})
}

func (a *ConnectionAPI) RenameColumn(
	sessionID string,
	ns Namespace,
	table, column, name string,
) error {
	err := domain.RequireNames("table", table, "column", column, "new column", name)
	if err != nil {
		return wrapError(err)
	}

	return a.changeSchema(sessionID, func(ctx context.Context, schema driver.Schema) error {
		return schema.RenameColumn(ctx, ns.toDriver(), table, column, name)
	})
}

func (a *ConnectionAPI) DropColumn(
	sessionID string,
	ns Namespace,
	table, column string,
) error {
	if err := domain.RequireNames("table", table, "column", column); err != nil {
		return wrapError(err)
	}

	return a.changeSchema(sessionID, func(ctx context.Context, schema driver.Schema) error {
		return schema.DropColumn(ctx, ns.toDriver(), table, column)
	})
}

// SetPrimaryKey replaces the table's key with the named columns, and drops it for an empty list.
func (a *ConnectionAPI) SetPrimaryKey(
	sessionID string,
	ns Namespace,
	table string,
	columns []string,
) error {
	if err := domain.RequireName("table", table); err != nil {
		return wrapError(err)
	}
	for _, column := range columns {
		if err := domain.RequireName("key column", column); err != nil {
			return wrapError(err)
		}
	}

	return a.changeSchema(sessionID, func(ctx context.Context, schema driver.Schema) error {
		return schema.SetPrimaryKey(ctx, ns.toDriver(), table, columns)
	})
}

// DropSource removes one view, routine or trigger. It takes the whole reference for the reason the
// driver method does: on three of the four kinds a name alone does not name one.
func (a *ConnectionAPI) DropSource(sessionID string, ns Namespace, ref SourceRef) error {
	// The kind is checked before it is quoted into the message, so a word that came off the wire
	// as something else is answered as the kind it is not rather than echoed back into a DROP.
	wanted, err := ref.toDriver()
	if err != nil {
		return wrapError(err)
	}
	if err := domain.RequireName(string(wanted.Kind), wanted.Name); err != nil {
		return wrapError(err)
	}

	return a.changeSchema(sessionID, func(ctx context.Context, schema driver.Schema) error {
		return schema.DropSource(ctx, ns.toDriver(), wanted)
	})
}

// LoadSource reads one view or one routine as the statement that declares it, which is what the
// designer opens on.
func (a *ConnectionAPI) LoadSource(sessionID string, ns Namespace, ref SourceRef) (Source, error) {
	schema, err := a.sessions.Schema(sessionID)
	if err != nil {
		return Source{}, wrapError(err)
	}
	// The kind is checked before it is quoted into the message, so a word that came off the wire
	// as something else is answered as the kind it is not rather than echoed back.
	wanted, err := ref.toDriver()
	if err != nil {
		return Source{}, wrapError(err)
	}
	if err := domain.RequireName(string(wanted.Kind), wanted.Name); err != nil {
		return Source{}, wrapError(err)
	}

	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	source, err := schema.Source(ctx, ns.toDriver(), wanted)
	if err != nil {
		return Source{}, wrapError(err)
	}
	return toSourceDTO(source), nil
}

// NewSource is the declaration a reader starts from when they are declaring one of these rather
// than altering it. It runs nothing and asks no server anything — it is the engine's own shape for
// that kind of object — and an engine that has none answers with an empty string, which is what
// the tab reads as there being nothing to start from.
func (a *ConnectionAPI) NewSource(sessionID string, ns Namespace, ref SourceRef) (string, error) {
	schema, err := a.sessions.Schema(sessionID)
	if err != nil {
		return "", wrapError(err)
	}

	wanted, err := ref.toDriver()
	if err != nil {
		return "", wrapError(err)
	}
	return schema.NewSource(ns.toDriver(), wanted), nil
}

// PreviewSource is exactly what SaveSource will run, so the editor's Changes pane shows the
// statements rather than a guess at them — the arrangement PreviewCreateDatabase has, and the one
// that matters most here: a definition being replaced is **dropped first**, and the reader is owed
// that in front of them before they press Save.
//
// An editor with nothing in it previews nothing rather than failing, since a blank buffer is a
// question not yet asked.
func (a *ConnectionAPI) PreviewSource(
	sessionID string,
	ns Namespace,
	save SourceSaveInput,
) ([]string, error) {
	schema, err := a.sessions.Schema(sessionID)
	if err != nil {
		return nil, wrapError(err)
	}

	planned, err := save.toDriver()
	if err != nil {
		return nil, wrapError(err)
	}

	statements := schema.SourceStatements(ns.toDriver(), planned)
	if statements == nil {
		return []string{}, nil
	}
	return statements, nil
}

// SaveSource runs what PreviewSource showed. A blank definition is refused here rather than at the
// server: it would otherwise spell the drop alone, which is the one thing a Save must never mean.
func (a *ConnectionAPI) SaveSource(sessionID string, ns Namespace, save SourceSaveInput) error {
	if strings.TrimSpace(save.Definition) == "" {
		return wrapError(domain.ValidationError{
			Field:   "definition",
			Message: "must not be empty",
		})
	}

	planned, err := save.toDriver()
	if err != nil {
		return wrapError(err)
	}

	return a.changeSchema(sessionID, func(ctx context.Context, schema driver.Schema) error {
		return schema.SaveSource(ctx, ns.toDriver(), planned)
	})
}

// LoadRoutineParams reads what a routine takes, which is what the Run pane draws a box per. It is
// a catalogue read like a table's shape, and an engine with no stored routines answers
// `UNSUPPORTED` naming it rather than an empty list the pane would draw as "it takes nothing".
func (a *ConnectionAPI) LoadRoutineParams(
	sessionID string,
	ns Namespace,
	ref SourceRef,
) ([]RoutineParam, error) {
	schema, err := a.sessions.Schema(sessionID)
	if err != nil {
		return nil, wrapError(err)
	}

	wanted, err := ref.toRoutine()
	if err != nil {
		return nil, wrapError(err)
	}
	if err := domain.RequireName(string(wanted.Kind), wanted.Name); err != nil {
		return nil, wrapError(err)
	}

	ctx, cancel := a.withTimeout(queryTimeout)
	defer cancel()

	params, err := schema.RoutineParams(ctx, ns.toDriver(), wanted)
	if err != nil {
		return nil, wrapError(err)
	}
	return toRoutineParamDTOs(params), nil
}

// PreviewCall is exactly what CallRoutine will run, so the pane shows the statements rather than a
// guess at them — the arrangement PreviewSource has, and it matters here for a second reason: what
// a procedure does to a reader's rows is the procedure's own business, and the call in front of
// them is the last thing they see before it happens.
//
// A call this engine has no grammar for previews nothing rather than failing, which is what the
// pane reads as there being nothing to run.
func (a *ConnectionAPI) PreviewCall(
	sessionID string,
	ns Namespace,
	call RoutineCallInput,
) ([]string, error) {
	schema, err := a.sessions.Schema(sessionID)
	if err != nil {
		return nil, wrapError(err)
	}

	planned, err := call.toDriver()
	if err != nil {
		return nil, wrapError(err)
	}

	statements := schema.CallStatements(ns.toDriver(), planned)
	if statements == nil {
		return []string{}, nil
	}
	return statements, nil
}

// CallRoutine runs what PreviewCall showed and answers with what the routine gave back: its rows,
// or the values its OUT parameters hold, or the count of what it changed.
//
// It takes a statement's timeout rather than a schema change's, because that is what it is — a
// routine is the reader's own program and takes as long as it takes.
func (a *ConnectionAPI) CallRoutine(
	sessionID string,
	ns Namespace,
	call RoutineCallInput,
) (QueryResult, error) {
	schema, err := a.sessions.Schema(sessionID)
	if err != nil {
		return QueryResult{}, wrapError(err)
	}

	planned, err := call.toDriver()
	if err != nil {
		return QueryResult{}, wrapError(err)
	}
	if err := domain.RequireName(string(planned.Kind), planned.Name); err != nil {
		return QueryResult{}, wrapError(err)
	}

	ctx, cancel := a.withTimeout(statementTimeout)
	defer cancel()

	started := time.Now()
	result, err := schema.CallRoutine(ctx, ns.toDriver(), planned)
	if err != nil {
		return QueryResult{}, wrapError(err)
	}
	return toQueryResultDTO(result, time.Since(started)), nil
}

func (a *ConnectionAPI) CreateIndex(
	sessionID string,
	ns Namespace,
	table string,
	spec IndexInput,
) error {
	if err := domain.RequireNames("table", table, "index", spec.Name); err != nil {
		return wrapError(err)
	}
	if err := requireColumns("index column", spec.Columns); err != nil {
		return wrapError(err)
	}

	return a.changeSchema(sessionID, func(ctx context.Context, schema driver.Schema) error {
		return schema.CreateIndex(ctx, ns.toDriver(), table, spec.toDriver())
	})
}

func (a *ConnectionAPI) CreateForeignKey(
	sessionID string,
	ns Namespace,
	table string,
	spec ForeignKeyInput,
) error {
	err := domain.RequireNames(
		"table", table,
		"foreign key", spec.Name,
		"referenced table", spec.ReferencedTable,
	)
	if err != nil {
		return wrapError(err)
	}
	if err := requireColumns("foreign key column", spec.Columns); err != nil {
		return wrapError(err)
	}
	if err := requireColumns("referenced column", spec.ReferencedColumns); err != nil {
		return wrapError(err)
	}
	// A key pairs its columns with the ones it points at, so a list of a different length
	// names no pairing at all.
	if len(spec.Columns) != len(spec.ReferencedColumns) {
		return wrapError(domain.ValidationError{
			Field:   "referenced column",
			Message: "must name one column for each column of the key",
		})
	}

	return a.changeSchema(sessionID, func(ctx context.Context, schema driver.Schema) error {
		return schema.CreateForeignKey(ctx, ns.toDriver(), table, spec.toDriver())
	})
}

func (a *ConnectionAPI) DropIndex(sessionID string, ns Namespace, table, index string) error {
	if err := domain.RequireNames("table", table, "index", index); err != nil {
		return wrapError(err)
	}

	return a.changeSchema(sessionID, func(ctx context.Context, schema driver.Schema) error {
		return schema.DropIndex(ctx, ns.toDriver(), table, index)
	})
}

func (a *ConnectionAPI) DropForeignKey(
	sessionID string,
	ns Namespace,
	table, constraint string,
) error {
	if err := domain.RequireNames("table", table, "foreign key", constraint); err != nil {
		return wrapError(err)
	}

	return a.changeSchema(sessionID, func(ctx context.Context, schema driver.Schema) error {
		return schema.DropForeignKey(ctx, ns.toDriver(), table, constraint)
	})
}

// AddPartition declares one more piece of a partitioned table. The bounds are the reader's own
// clause and are required: a piece that says what it holds is the whole of what declaring one is,
// and an empty clause would be a statement the server refuses for a reason nobody could read.
func (a *ConnectionAPI) AddPartition(
	sessionID string,
	ns Namespace,
	table string,
	spec PartitionInput,
) error {
	if err := domain.RequireNames("table", table, "partition", spec.Name); err != nil {
		return wrapError(err)
	}
	if strings.TrimSpace(spec.Bounds) == "" {
		return wrapError(domain.ValidationError{
			Field:   "bounds",
			Message: "must say what the partition holds",
		})
	}

	return a.changeSchema(sessionID, func(ctx context.Context, schema driver.Schema) error {
		return schema.AddPartition(ctx, ns.toDriver(), table, spec.toDriver())
	})
}

// DropPartition removes a piece **and the rows in it**; TruncatePartition removes the rows alone.
// The difference is the one DropTable and TruncateTable have, one level down, and it is the reason
// they are two methods rather than one with a flag.
func (a *ConnectionAPI) DropPartition(
	sessionID string,
	ns Namespace,
	table, partition string,
) error {
	if err := domain.RequireNames("table", table, "partition", partition); err != nil {
		return wrapError(err)
	}

	return a.changeSchema(sessionID, func(ctx context.Context, schema driver.Schema) error {
		return schema.DropPartition(ctx, ns.toDriver(), table, partition)
	})
}

func (a *ConnectionAPI) TruncatePartition(
	sessionID string,
	ns Namespace,
	table, partition string,
) error {
	if err := domain.RequireNames("table", table, "partition", partition); err != nil {
		return wrapError(err)
	}

	return a.changeSchema(sessionID, func(ctx context.Context, schema driver.Schema) error {
		return schema.TruncatePartition(ctx, ns.toDriver(), table, partition)
	})
}

// RefreshMaterializedView recomputes the rows such a view stores. It is a schema change rather than
// a read even though it changes no shape: it runs for as long as the query behind it takes, and it
// is the one thing done to a materialized view that is not done to an ordinary one.
func (a *ConnectionAPI) RefreshMaterializedView(sessionID string, ns Namespace, name string) error {
	if err := domain.RequireName("materialized view", name); err != nil {
		return wrapError(err)
	}

	return a.changeSchema(sessionID, func(ctx context.Context, schema driver.Schema) error {
		return schema.RefreshMaterializedView(ctx, ns.toDriver(), name)
	})
}

// changeSchema is the shape every schema change shares: find the session, give the statement
// its own deadline, and wrap whatever comes back. Only the statement itself differs.
func (a *ConnectionAPI) changeSchema(
	sessionID string,
	change func(ctx context.Context, schema driver.Schema) error,
) error {
	schema, err := a.sessions.Schema(sessionID)
	if err != nil {
		return wrapError(err)
	}

	ctx, cancel := a.withTimeout(schemaTimeout)
	defer cancel()

	return wrapError(change(ctx, schema))
}

// requireColumn refuses a column that named neither itself nor a type. A default is allowed to
// be missing, which is what a column with no default means.
func requireColumn(column ColumnInput) error {
	return domain.RequireNames("column", column.Name, "column type", column.Type)
}

// requireColumns refuses a list that named nothing, which is what an index or a key over no
// column at all would be.
func requireColumns(what string, columns []string) error {
	if len(columns) == 0 {
		return domain.NameError{What: what}
	}
	for _, column := range columns {
		if err := domain.RequireName(what, column); err != nil {
			return err
		}
	}
	return nil
}
