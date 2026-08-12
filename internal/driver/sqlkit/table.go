package sqlkit

import (
	"context"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/driver"
)

// Data is the half of a session that reads and writes *rows*, and it is here for the reason DDL is:
// none of it varies by engine once the quoting, the paging and the placeholders are the Dialect's.
// An adapter embeds it and supplies the three things only it can answer — a table's shape, the pool
// a read runs on, and its own error map — and writes no SELECT, UPDATE, INSERT or DELETE of its own.
//
// Every statement here is built from the columns the *catalogue* reported, so a name a reader typed
// is checked against a real column before it is quoted into anything: see Where and OrderBy, which
// drop or refuse a name the table has not got.
type Data struct {
	Dialect Dialect
	// Columns reads a table's declared shape. It is the adapter's because a catalogue is, and it is
	// the one read every statement below begins with.
	Columns func(ctx context.Context, ns driver.Namespace, table string) ([]driver.Column, error)
	// Open hands back the pool a statement runs on. The database is what an engine with a pool per
	// database needs and what an engine with one pool ignores, exactly as DDL.Run's is.
	Open func(database string) (Queryer, error)
	// Fail is the adapter's error map, so nothing here has to know how this engine spells a failure.
	Fail func(error) error
	// Tx is the session's manual transaction, and nil is a session that cannot hold one. While one
	// is open every read and every write below runs *inside* it — which is what makes the grid show
	// the reader their own uncommitted rows, those rows existing on that one connection and nowhere
	// else. See queryer, and see Txn.
	Tx *Txn
}

// queryer is where this session's statements currently go: the open transaction, or the pool the
// adapter hands back. It is the one seam, so no method below has to know a transaction exists.
func (t Data) queryer(database string) (Queryer, error) {
	if open := t.Tx.Queryer(); open != nil {
		return open, nil
	}
	return t.Open(database)
}

func (t Data) TableRows(ctx context.Context, page driver.TablePage) (driver.TableRows, error) {
	columns, err := t.Columns(ctx, page.Namespace, page.Table)
	if err != nil {
		return driver.TableRows{}, err
	}

	where, err := t.Dialect.Where(page.Filter, columns)
	if err != nil {
		return driver.TableRows{}, err
	}

	// The total is counted through the same filters the rows are read through, or the pager would
	// walk pages of a table the reader has narrowed away.
	total, err := t.count(ctx, page.Namespace, page.Table, where)
	if err != nil {
		return driver.TableRows{}, err
	}

	db, err := t.queryer(page.Database)
	if err != nil {
		return driver.TableRows{}, err
	}

	query := t.Dialect.SelectPage(page, columns, where)

	// The statement itself bounds the read — either through its page clause or because the caller
	// asked for every row — so no cap is applied here. A page carrying a stream takes its rows as
	// they arrive and the result then carries none.
	result, err := Read(ctx, db, query, 0, page.Stream)
	if err != nil {
		return driver.TableRows{}, t.Fail(err)
	}

	return driver.TableRows{
		Columns:   columns,
		Rows:      result.Rows,
		Total:     total,
		Statement: query,
	}, nil
}

func (t Data) ColumnValues(ctx context.Context, pick driver.ColumnPick) ([]string, error) {
	columns, err := t.Columns(ctx, pick.Namespace, pick.Table)
	if err != nil {
		return nil, err
	}

	query, err := t.Dialect.DistinctValues(
		t.Dialect.Table(pick.Namespace, pick.Table),
		pick.Column,
		columns,
		pick.Limit,
	)
	if err != nil {
		return nil, err
	}

	db, err := t.queryer(pick.Database)
	if err != nil {
		return nil, err
	}

	// The statement's own bound bounds the read, exactly as a page's does, so no cap is applied
	// here and nothing is streamed: the answer is a list somebody scrolls.
	result, err := Read(ctx, db, query, 0, nil)
	if err != nil {
		return nil, t.Fail(err)
	}
	return FirstColumn(result.Rows), nil
}

func (t Data) MatchingRows(
	ctx context.Context,
	search driver.RowSearch,
) (driver.RowMatches, error) {
	columns, err := t.Columns(ctx, search.Namespace, search.Table)
	if err != nil {
		return driver.RowMatches{}, err
	}

	where, err := t.Dialect.MatchAny(search.Match, columns)
	if err != nil {
		return driver.RowMatches{}, err
	}

	total, err := t.count(ctx, search.Namespace, search.Table, where)
	if err != nil {
		return driver.RowMatches{}, err
	}

	table := t.Dialect.Table(search.Namespace, search.Table)
	return driver.RowMatches{Count: total, Statement: t.Dialect.SelectMatching(table, where)}, nil
}

func (t Data) UpdateCell(ctx context.Context, edit driver.CellEdit) error {
	columns, err := t.Columns(ctx, edit.Namespace, edit.Table)
	if err != nil {
		return err
	}

	keys, err := EditableKeys(columns, edit.Key)
	if err != nil {
		return err
	}

	query, args := t.Dialect.UpdateCell(edit, keys)
	return t.exec(ctx, edit.Database, query, args)
}

func (t Data) InsertRow(ctx context.Context, insert driver.RowInsert) error {
	columns, err := t.Columns(ctx, insert.Namespace, insert.Table)
	if err != nil {
		return err
	}

	query, args := t.Dialect.InsertRow(insert, columns)
	return t.exec(ctx, insert.Database, query, args)
}

func (t Data) DeleteRow(ctx context.Context, removal driver.RowDelete) error {
	columns, err := t.Columns(ctx, removal.Namespace, removal.Table)
	if err != nil {
		return err
	}

	keys, err := EditableKeys(columns, removal.Key)
	if err != nil {
		return err
	}

	query, args := t.Dialect.DeleteRow(removal, keys)
	return t.exec(ctx, removal.Database, query, args)
}

func (t Data) exec(ctx context.Context, database, query string, args []any) error {
	db, err := t.queryer(database)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, query, args...); err != nil {
		return t.Fail(err)
	}
	return nil
}

func (t Data) count(
	ctx context.Context,
	ns driver.Namespace,
	table, where string,
) (int64, error) {
	db, err := t.queryer(ns.Database)
	if err != nil {
		return 0, err
	}

	query := "SELECT COUNT(*) FROM " + t.Dialect.Table(ns, table) + where

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return 0, t.Fail(err)
	}
	defer rows.Close()

	var total int64
	if !rows.Next() {
		return 0, t.Fail(rows.Err())
	}
	if err := rows.Scan(&total); err != nil {
		return 0, t.Fail(err)
	}
	return total, nil
}

// SelectPage spells the whole read of one window onto a table. The columns are named rather than
// starred so the header the grid draws is the shape the catalogue reported, in that order, whatever
// the engine would have chosen; the WHERE arrives already built, because the count needs the same
// one; and the bounds are Page's, which is where the two spellings of a page live.
func (d Dialect) SelectPage(
	page driver.TablePage,
	columns []driver.Column,
	where string,
) string {
	names := make([]string, len(columns))
	for i, column := range columns {
		names[i] = d.QuoteIdent(column.Name)
	}

	return "SELECT " + strings.Join(names, ", ") +
		" FROM " + d.Table(page.Namespace, page.Table) +
		where +
		d.Page(d.OrderBy(page.Sort, columns), page.Limit, page.Offset)
}

// UpdateCell writes one cell of one stored row. The value is bound rather than quoted in — this is
// not a statement anybody is shown — and the key columns follow it in the table's own order, which
// is what makes the arguments line up with the placeholders whichever way the engine numbers them.
func (d Dialect) UpdateCell(edit driver.CellEdit, keys []string) (string, []any) {
	args := make([]any, 0, len(keys)+1)
	args = append(args, edit.Value)

	conditions := make([]string, len(keys))
	for i, key := range keys {
		conditions[i] = d.QuoteIdent(key) + " = " + d.Placeholder(len(args)+1)
		args = append(args, edit.Key[key])
	}

	query := "UPDATE " + d.Table(edit.Namespace, edit.Table) +
		" SET " + d.QuoteIdent(edit.Column) + " = " + d.Placeholder(1) +
		" WHERE " + strings.Join(conditions, " AND ") +
		d.writeLimit()

	return query, args
}

// InsertRow names only the columns the new row actually filled in, so a column left alone takes the
// server's own default. The order is the table's rather than the map's, which has none.
//
// A row that filled in nothing is the one place the engines have three answers: MySQL's empty
// column list, the DEFAULT VALUES the standard gives, and — for an engine with neither — naming
// every column and giving each of them the keyword.
func (d Dialect) InsertRow(insert driver.RowInsert, columns []driver.Column) (string, []any) {
	names := make([]string, 0, len(insert.Values))
	holders := make([]string, 0, len(insert.Values))
	args := make([]any, 0, len(insert.Values))

	for _, column := range columns {
		value, ok := insert.Values[column.Name]
		if !ok {
			continue
		}
		names = append(names, d.QuoteIdent(column.Name))
		holders = append(holders, d.Placeholder(len(args)+1))
		args = append(args, value)
	}

	target := "INSERT INTO " + d.Table(insert.Namespace, insert.Table)
	if len(names) == 0 {
		return target + d.everyDefault(columns), nil
	}

	return target +
		" (" + strings.Join(names, ", ") + ")" +
		" VALUES (" + strings.Join(holders, ", ") + ")", args
}

func (d Dialect) DeleteRow(removal driver.RowDelete, keys []string) (string, []any) {
	args := make([]any, 0, len(keys))

	conditions := make([]string, len(keys))
	for i, key := range keys {
		conditions[i] = d.QuoteIdent(key) + " = " + d.Placeholder(len(args)+1)
		args = append(args, removal.Key[key])
	}

	return "DELETE FROM " + d.Table(removal.Namespace, removal.Table) +
		" WHERE " + strings.Join(conditions, " AND ") +
		d.writeLimit(), args
}

// writeLimit bounds a write to one row where the engine will take such a bound. It is belt and
// braces: the primary key in the WHERE already names at most one row, and the engines without it
// are none the worse for that.
func (d Dialect) writeLimit() string {
	if d.LimitsWrite {
		return " LIMIT 1"
	}
	return ""
}

// everyDefault is a new row that filled nothing in. See InsertRow for the three spellings; an
// engine that has neither of the first two names every column and gives each the keyword.
func (d Dialect) everyDefault(columns []driver.Column) string {
	if d.EmptyInsert != "" {
		return d.EmptyInsert
	}

	names := make([]string, 0, len(columns))
	defaults := make([]string, 0, len(columns))
	for _, column := range columns {
		names = append(names, d.QuoteIdent(column.Name))
		defaults = append(defaults, "DEFAULT")
	}
	return " (" + strings.Join(names, ", ") + ") VALUES (" + strings.Join(defaults, ", ") + ")"
}
