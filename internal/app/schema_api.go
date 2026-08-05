package app

import (
	"context"
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

func (a *ConnectionAPI) DropView(sessionID string, ns Namespace, view string) error {
	if err := domain.RequireName("view", view); err != nil {
		return wrapError(err)
	}

	return a.changeSchema(sessionID, func(ctx context.Context, schema driver.Schema) error {
		return schema.DropView(ctx, ns.toDriver(), view)
	})
}

func (a *ConnectionAPI) DropRoutine(sessionID string, ns Namespace, routine RoutineInfo) error {
	if err := domain.RequireName("routine", routine.Name); err != nil {
		return wrapError(err)
	}

	return a.changeSchema(sessionID, func(ctx context.Context, schema driver.Schema) error {
		return schema.DropRoutine(ctx, ns.toDriver(), routine.toDriver())
	})
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
