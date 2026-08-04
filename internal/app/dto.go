package app

import (
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

const (
	DefaultPageSize  = 100
	DefaultQueryRows = 1000

	// ColumnValueLimit caps the values the filter builder offers to pick from. A column of a
	// million rows is not a list anybody scrolls, and the box beside the list takes whatever the
	// list does not hold.
	ColumnValueLimit = 200

	// AllRows asks for every row rather than a page or a capped read. Bounded reads are what
	// keep the memory numbers honest, so lifting the bound is opt-in from a toolbar and never
	// a default: a missing or nonsensical count still falls back to the default above, which
	// means the Go zero value cannot accidentally mean unbounded.
	AllRows = -1
)

type Profile struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Engine       string `json:"engine"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	User         string `json:"user"`
	Database     string `json:"database"`
	TLSMode      string `json:"tlsMode"`
	SavePassword bool   `json:"savePassword"`
	HasPassword  bool   `json:"hasPassword"`
}

type ProfileInput struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Engine       string `json:"engine"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	User         string `json:"user"`
	Password     string `json:"password"`
	Database     string `json:"database"`
	TLSMode      string `json:"tlsMode"`
	SavePassword bool   `json:"savePassword"`
}

type TestResult struct {
	ServerVersion string `json:"serverVersion"`
	LatencyMs     int64  `json:"latencyMs"`
}

type Session struct {
	ID            string `json:"id"`
	ProfileID     string `json:"profileId"`
	Target        string `json:"target"`
	ServerVersion string `json:"serverVersion"`
}

// Namespace is where a table lives. Schema is empty on MySQL, whose database is its schema.
type Namespace struct {
	Database string `json:"database"`
	Schema   string `json:"schema"`
}

// TableSortInput is the column the reader sorted by. An empty Column means none, and the
// table falls back to its primary key.
type TableSortInput struct {
	Column     string `json:"column"`
	Descending bool   `json:"descending"`
}

// criterionGroup is the kind a TableCriterionInput carries when it is parentheses rather than a
// condition. Anything else is read as a condition, which the driver then refuses if its operator
// is one no engine knows — so one place says no rather than two.
const criterionGroup = "group"

// TableCriterionInput is one node of the tree the reader narrowed a read by, flattened for the
// wire: kind says whether it is a condition on one column or a group of further nodes, and the
// fields belonging to the other kind go unread. One struct rather than two because JSON carries
// no union, and the frontend's own Criterion is this same shape.
//
// Operator is a token from a fixed list rather than the operator spelled out — see
// driver.TableFilterOp — and how many values it takes is its own business.
type TableCriterionInput struct {
	Kind        string                `json:"kind"`
	Column      string                `json:"column"`
	Operator    string                `json:"operator"`
	Values      []string              `json:"values"`
	Conjunction string                `json:"conjunction"`
	Children    []TableCriterionInput `json:"children"`
}

type TablePageInput struct {
	Namespace
	Table string `json:"table"`
	// RunID is minted by the caller so that a read can be stopped before it answers, and so the
	// batches of a streamed read can be told from another tab's. An empty one asks for an
	// ordinary read, bounded by a deadline and answered whole — see LoadTableRows.
	RunID  string           `json:"runId"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
	Sort   []TableSortInput `json:"sort"`
	// Filter is the root of the condition tree, and is therefore always a group.
	Filter TableCriterionInput `json:"filter"`
}

// ColumnValuesInput names the column a filter's value is picked from. The cap on the answer is
// this side's rather than the caller's — see ColumnValueLimit.
type ColumnValuesInput struct {
	Namespace
	Table  string `json:"table"`
	Column string `json:"column"`
}

type TableColumn struct {
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	FullType   string  `json:"fullType"`
	Nullable   bool    `json:"nullable"`
	PrimaryKey bool    `json:"primaryKey"`
	Default    *string `json:"default"`
}

// TableRows is one page of a table. `statement` is the read that produced it, exactly as it ran,
// so the status line under the grid can show it and a reader can copy it into an editor.
type TableRows struct {
	Columns   []TableColumn `json:"columns"`
	Rows      [][]*string   `json:"rows"`
	Total     int64         `json:"total"`
	Limit     int           `json:"limit"`
	Offset    int           `json:"offset"`
	Statement string        `json:"statement"`
}

// TableList is one row per table of a namespace: its name in the first column, then whatever
// else the engine's catalogue keeps about it — see driver.TableList for why the columns are the
// engine's own rather than a fixed set.
type TableList struct {
	Columns []TableColumn `json:"columns"`
	Rows    [][]*string   `json:"rows"`
}

type TableSchema struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
}

// RoutineInfo is one stored function or procedure. It travels back into a drop unchanged,
// because on PostgreSQL the name alone does not name one — two functions may share it.
type RoutineInfo struct {
	Name      string `json:"name"`
	Procedure bool   `json:"procedure"`
	Arguments string `json:"arguments"`
}

// Objects is everything the tree draws a folder for under a namespace, read in one call
// because opening any one folder needs every folder's count.
type Objects struct {
	Tables   []string      `json:"tables"`
	Views    []string      `json:"views"`
	Routines []RoutineInfo `json:"routines"`
}

type IndexInfo struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique"`
	Primary bool     `json:"primary"`
	Method  string   `json:"method"`
}

type ForeignKeyInfo struct {
	Name              string   `json:"name"`
	Columns           []string `json:"columns"`
	ReferencedSchema  string   `json:"referencedSchema"`
	ReferencedTable   string   `json:"referencedTable"`
	ReferencedColumns []string `json:"referencedColumns"`
	OnUpdate          string   `json:"onUpdate"`
	OnDelete          string   `json:"onDelete"`
}

// TableStructure is a table's shape rather than its rows: what the tree draws under a table
// and what the designer shows.
type TableStructure struct {
	Columns     []TableColumn    `json:"columns"`
	Indexes     []IndexInfo      `json:"indexes"`
	ForeignKeys []ForeignKeyInfo `json:"foreignKeys"`
}

// IndexInput is an index as the designer declares it. `method` reaches the statement verbatim,
// the way a column's type does, and an empty one takes the engine's own default.
type IndexInput struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique"`
	Method  string   `json:"method"`
}

// ForeignKeyInput is a referential constraint as the designer declares it. `referencedSchema`
// is whichever part of a namespace the engine qualifies a table by, and is empty for a table in
// the same namespace as the one holding the key.
type ForeignKeyInput struct {
	Name              string   `json:"name"`
	Columns           []string `json:"columns"`
	ReferencedSchema  string   `json:"referencedSchema"`
	ReferencedTable   string   `json:"referencedTable"`
	ReferencedColumns []string `json:"referencedColumns"`
	OnUpdate          string   `json:"onUpdate"`
	OnDelete          string   `json:"onDelete"`
}

// ColumnInput is a column as the dialog declares it. Type carries its length and scale —
// `varchar(255)`, `numeric(10,2)` — and Default is an expression, since that is what a
// server-side default is; both reach the statement verbatim, DDL having nothing to bind.
type ColumnInput struct {
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	Nullable   bool    `json:"nullable"`
	Default    *string `json:"default"`
	PrimaryKey bool    `json:"primaryKey"`
}

type TableSpecInput struct {
	Namespace
	Table   string        `json:"table"`
	Columns []ColumnInput `json:"columns"`
}

// DatabaseSpecInput is a CREATE DATABASE as the dialog declares it. A field an engine has no
// concept of arrives blank rather than being left out of the shape, since one dialog fills it
// for both and the driver is what decides which parts it reads.
type DatabaseSpecInput struct {
	Name             string   `json:"name"`
	Owner            string   `json:"owner"`
	Template         string   `json:"template"`
	Encoding         string   `json:"encoding"`
	Collation        string   `json:"collation"`
	CType            string   `json:"ctype"`
	Tablespace       string   `json:"tablespace"`
	ConnectionLimit  int      `json:"connectionLimit"`
	AllowConnections bool     `json:"allowConnections"`
	IsTemplate       bool     `json:"isTemplate"`
	Comment          string   `json:"comment"`
	Extensions       []string `json:"extensions"`
}

// DatabaseOptions is what the New Database dialog fills its lists from. An engine answers with
// nothing for a list it has no such concept of, which is how the dialog knows not to draw it.
type DatabaseOptions struct {
	Owners      []string `json:"owners"`
	Templates   []string `json:"templates"`
	Encodings   []string `json:"encodings"`
	Collations  []string `json:"collations"`
	Tablespaces []string `json:"tablespaces"`
	Extensions  []string `json:"extensions"`
}

type QueryInput struct {
	Namespace
	RunID   string `json:"runId"`
	SQL     string `json:"sql"`
	MaxRows int    `json:"maxRows"`
	Explain bool   `json:"explain"`
}

// QueryResult carries both shapes of outcome: a statement that returns rows fills
// Columns, and one that does not leaves them empty and reports RowsAffected.
// The namespace and Table name the source the rows may be written back to, and are empty
// for a result the driver could not trace to a single table.
type QueryResult struct {
	Namespace
	Columns      []TableColumn `json:"columns"`
	Rows         [][]*string   `json:"rows"`
	RowsAffected int64         `json:"rowsAffected"`
	Truncated    bool          `json:"truncated"`
	ElapsedMs    int64         `json:"elapsedMs"`
	Table        string        `json:"table"`
}

// StatementOutcome is what one statement of a script did. `error` is the server's own wording
// and is empty for one that succeeded, so a script's messages read as the output panel does.
type StatementOutcome struct {
	SQL          string `json:"sql"`
	Rows         int    `json:"rows"`
	RowsAffected int64  `json:"rowsAffected"`
	ElapsedMs    int64  `json:"elapsedMs"`
	Error        string `json:"error"`
	ErrorCode    string `json:"errorCode"`
}

// ScriptResult is one Execute: what each statement did, and the rows of the last one that
// returned any — which is what the grid shows.
type ScriptResult struct {
	QueryResult
	Statements []StatementOutcome `json:"statements"`
}

// HistoryEntry is one Execute the reader asked for, as the History pane lists it. `ranAt` is epoch
// milliseconds — the clock the rest of the wire counts in — so the pane draws it in the reader's
// own time zone rather than in whatever one the log was written in.
type HistoryEntry struct {
	Namespace
	ProfileID    string `json:"profileId"`
	SQL          string `json:"sql"`
	Rows         int    `json:"rows"`
	RowsAffected int64  `json:"rowsAffected"`
	ElapsedMs    int64  `json:"elapsedMs"`
	ErrorCode    string `json:"errorCode"`
	Error        string `json:"error"`
	Runs         int    `json:"runs"`
	RanAt        int64  `json:"ranAt"`
}

type CellEditInput struct {
	Namespace
	Table  string            `json:"table"`
	Column string            `json:"column"`
	Value  *string           `json:"value"`
	Key    map[string]string `json:"key"`
}

type RowValueInput struct {
	Column string  `json:"column"`
	Value  *string `json:"value"`
}

// RowInsertInput carries only the columns a new row filled in, so an untouched column takes
// the server's default rather than an explicit NULL. The values are a list rather than a map
// because Wails' binding generator cannot express a map of nullable values.
type RowInsertInput struct {
	Namespace
	Table  string          `json:"table"`
	Values []RowValueInput `json:"values"`
}

type RowDeleteInput struct {
	Namespace
	Table string            `json:"table"`
	Key   map[string]string `json:"key"`
}

func toProfileDTO(profile domain.ConnectionProfile, hasPassword bool) Profile {
	return Profile{
		ID:           profile.ID,
		Name:         profile.Name,
		Engine:       string(profile.Engine),
		Host:         profile.Host,
		Port:         profile.Port,
		User:         profile.User,
		Database:     profile.Database,
		TLSMode:      string(profile.TLSMode),
		SavePassword: profile.SavePassword,
		HasPassword:  hasPassword,
	}
}

func (n Namespace) toDriver() driver.Namespace {
	return driver.Namespace{Database: n.Database, Schema: n.Schema}
}

func toNamespaceDTO(ns driver.Namespace) Namespace {
	return Namespace{Database: ns.Database, Schema: ns.Schema}
}

func (in TablePageInput) toDriver() driver.TablePage {
	sort := make([]driver.TableSort, len(in.Sort))
	for i, key := range in.Sort {
		sort[i] = driver.TableSort{Column: key.Column, Descending: key.Descending}
	}

	return driver.TablePage{
		Namespace: in.Namespace.toDriver(),
		Table:     in.Table,
		Limit:     driverRows(in.Limit, DefaultPageSize),
		Offset:    max(in.Offset, 0),
		Sort:      sort,
		Filter:    in.Filter.toDriverGroup(),
	}
}

func (in TableCriterionInput) toDriver() driver.TableCriterion {
	if in.Kind != criterionGroup {
		return driver.TableFilter{
			Column: in.Column,
			Op:     driver.TableFilterOp(in.Operator),
			Values: in.Values,
		}
	}

	children := make([]driver.TableCriterion, len(in.Children))
	for i, child := range in.Children {
		children[i] = child.toDriver()
	}

	return driver.TableGroup{
		Conjunction: driver.TableConjunction(in.Conjunction),
		Children:    children,
	}
}

// toDriverGroup reads the root of the tree, which is a group by construction — the frontend's own
// root is one. A node arriving as anything else is read as the group of nothing, which narrows
// nothing, exactly as a read that carried no filter at all does.
func (in TableCriterionInput) toDriverGroup() driver.TableGroup {
	group, _ := in.toDriver().(driver.TableGroup)
	return group
}

func (in QueryInput) toDriver() driver.Query {
	return driver.Query{
		Namespace: in.Namespace.toDriver(),
		SQL:       in.SQL,
		MaxRows:   driverRows(in.MaxRows, DefaultQueryRows),
		Explain:   in.Explain,
	}
}

// driverRows translates a wire row count into the driver's convention, where a non-positive
// limit reads everything. A typed count is passed through unclamped: a ceiling would be
// theatre next to an All option, and silently answering a different question than the one
// asked is worse than a slow grid.
func driverRows(rows, fallback int) int {
	switch {
	case rows == AllRows:
		return 0
	case rows <= 0:
		return fallback
	default:
		return rows
	}
}

func (in CellEditInput) toDriver() driver.CellEdit {
	return driver.CellEdit{
		Namespace: in.Namespace.toDriver(),
		Table:     in.Table,
		Column:    in.Column,
		Value:     in.Value,
		Key:       in.Key,
	}
}

func (in RowInsertInput) toDriver() driver.RowInsert {
	values := make(map[string]*string, len(in.Values))
	for _, value := range in.Values {
		values[value.Column] = value.Value
	}

	return driver.RowInsert{
		Namespace: in.Namespace.toDriver(),
		Table:     in.Table,
		Values:    values,
	}
}

func (in RowDeleteInput) toDriver() driver.RowDelete {
	return driver.RowDelete{
		Namespace: in.Namespace.toDriver(),
		Table:     in.Table,
		Key:       in.Key,
	}
}

func toTableRowsDTO(rows driver.TableRows, page driver.TablePage) TableRows {
	return TableRows{
		Columns:   toColumnDTOs(rows.Columns),
		Rows:      rows.Rows,
		Total:     rows.Total,
		Limit:     page.Limit,
		Offset:    page.Offset,
		Statement: rows.Statement,
	}
}

func toQueryResultDTO(result driver.QueryResult, elapsed time.Duration) QueryResult {
	return QueryResult{
		Namespace:    toNamespaceDTO(result.Namespace),
		Columns:      toColumnDTOs(result.Columns),
		Rows:         result.Rows,
		RowsAffected: result.RowsAffected,
		Truncated:    result.Truncated,
		ElapsedMs:    elapsed.Milliseconds(),
		Table:        result.Table,
	}
}

func toHistoryDTOs(entries []domain.HistoryEntry) []HistoryEntry {
	out := make([]HistoryEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, HistoryEntry{
			Namespace:    Namespace{Database: entry.Database, Schema: entry.Schema},
			ProfileID:    entry.ProfileID,
			SQL:          entry.SQL,
			Rows:         entry.Rows,
			RowsAffected: entry.RowsAffected,
			ElapsedMs:    entry.ElapsedMs,
			ErrorCode:    entry.ErrorCode,
			Error:        entry.Error,
			Runs:         entry.Runs,
			RanAt:        entry.RanAt.UnixMilli(),
		})
	}
	return out
}

func (in ColumnInput) toDriver() driver.ColumnSpec {
	return driver.ColumnSpec{
		Name:       in.Name,
		Type:       in.Type,
		Nullable:   in.Nullable,
		Default:    in.Default,
		PrimaryKey: in.PrimaryKey,
	}
}

func (in DatabaseSpecInput) toDriver() driver.DatabaseSpec {
	return driver.DatabaseSpec{
		Name:             in.Name,
		Owner:            in.Owner,
		Template:         in.Template,
		Encoding:         in.Encoding,
		Collation:        in.Collation,
		CType:            in.CType,
		Tablespace:       in.Tablespace,
		ConnectionLimit:  in.ConnectionLimit,
		AllowConnections: in.AllowConnections,
		IsTemplate:       in.IsTemplate,
		Comment:          in.Comment,
		Extensions:       in.Extensions,
	}
}

func toDatabaseOptionsDTO(source driver.DatabaseOptions) DatabaseOptions {
	return DatabaseOptions{
		Owners:      source.Owners,
		Templates:   source.Templates,
		Encodings:   source.Encodings,
		Collations:  source.Collations,
		Tablespaces: source.Tablespaces,
		Extensions:  source.Extensions,
	}
}

func (in TableSpecInput) toDriver() driver.TableSpec {
	columns := make([]driver.ColumnSpec, 0, len(in.Columns))
	for _, column := range in.Columns {
		columns = append(columns, column.toDriver())
	}

	return driver.TableSpec{
		Namespace: in.Namespace.toDriver(),
		Table:     in.Table,
		Columns:   columns,
	}
}

func toTableStructureDTO(source driver.TableStructure) TableStructure {
	indexes := make([]IndexInfo, 0, len(source.Indexes))
	for _, index := range source.Indexes {
		indexes = append(indexes, IndexInfo{
			Name:    index.Name,
			Columns: index.Columns,
			Unique:  index.Unique,
			Primary: index.Primary,
			Method:  index.Method,
		})
	}

	keys := make([]ForeignKeyInfo, 0, len(source.ForeignKeys))
	for _, key := range source.ForeignKeys {
		keys = append(keys, ForeignKeyInfo{
			Name:              key.Name,
			Columns:           key.Columns,
			ReferencedSchema:  key.ReferencedSchema,
			ReferencedTable:   key.ReferencedTable,
			ReferencedColumns: key.ReferencedColumns,
			OnUpdate:          key.OnUpdate,
			OnDelete:          key.OnDelete,
		})
	}

	return TableStructure{
		Columns:     toColumnDTOs(source.Columns),
		Indexes:     indexes,
		ForeignKeys: keys,
	}
}

func (in IndexInput) toDriver() driver.IndexSpec {
	return driver.IndexSpec{
		Name:    in.Name,
		Columns: in.Columns,
		Unique:  in.Unique,
		Method:  in.Method,
	}
}

func (in ForeignKeyInput) toDriver() driver.ForeignKeySpec {
	return driver.ForeignKeySpec{
		Name:              in.Name,
		Columns:           in.Columns,
		ReferencedSchema:  in.ReferencedSchema,
		ReferencedTable:   in.ReferencedTable,
		ReferencedColumns: in.ReferencedColumns,
		OnUpdate:          in.OnUpdate,
		OnDelete:          in.OnDelete,
	}
}

func (in RoutineInfo) toDriver() driver.Routine {
	return driver.Routine{
		Name:      in.Name,
		Procedure: in.Procedure,
		Arguments: in.Arguments,
	}
}

func toObjectsDTO(source driver.Objects) Objects {
	routines := make([]RoutineInfo, 0, len(source.Routines))
	for _, routine := range source.Routines {
		routines = append(routines, RoutineInfo{
			Name:      routine.Name,
			Procedure: routine.Procedure,
			Arguments: routine.Arguments,
		})
	}

	return Objects{Tables: source.Tables, Views: source.Views, Routines: routines}
}

func toTableListDTO(source driver.TableList) TableList {
	return TableList{Columns: toColumnDTOs(source.Columns), Rows: source.Rows}
}

func toTableSchemaDTOs(source []driver.TableSchema) []TableSchema {
	tables := make([]TableSchema, 0, len(source))
	for _, table := range source {
		tables = append(tables, TableSchema{Name: table.Name, Columns: table.Columns})
	}
	return tables
}

func toColumnDTOs(source []driver.Column) []TableColumn {
	columns := make([]TableColumn, 0, len(source))
	for _, column := range source {
		columns = append(columns, TableColumn{
			Name:       column.Name,
			Type:       column.Type,
			FullType:   column.FullType,
			Nullable:   column.Nullable,
			PrimaryKey: column.PrimaryKey,
			Default:    column.Default,
		})
	}
	return columns
}

// toDomain fills in what the dialog may leave blank. An unnamed engine is MySQL, which is
// what a profile written before the second engine existed means, and the port then follows
// from the engine rather than from a constant.
func (in ProfileInput) toDomain() domain.ConnectionProfile {
	engine := domain.Engine(in.Engine)
	if engine == "" {
		engine = domain.EngineMySQL
	}

	port := in.Port
	if port == 0 {
		port = engine.DefaultPort()
	}

	tlsMode := domain.TLSMode(in.TLSMode)
	if tlsMode == "" {
		tlsMode = domain.TLSPreferred
	}

	return domain.ConnectionProfile{
		ID:           in.ID,
		Name:         in.Name,
		Engine:       engine,
		Host:         in.Host,
		Port:         port,
		User:         in.User,
		Database:     in.Database,
		TLSMode:      tlsMode,
		SavePassword: in.SavePassword,
	}
}
