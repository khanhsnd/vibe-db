package app

import (
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

const (
	DefaultPageSize  = 100
	DefaultQueryRows = 1000

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

type TablePageInput struct {
	Namespace
	Table  string `json:"table"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

type TableColumn struct {
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	FullType   string  `json:"fullType"`
	Nullable   bool    `json:"nullable"`
	PrimaryKey bool    `json:"primaryKey"`
	Default    *string `json:"default"`
}

type TableRows struct {
	Columns []TableColumn `json:"columns"`
	Rows    [][]*string   `json:"rows"`
	Total   int64         `json:"total"`
	Limit   int           `json:"limit"`
	Offset  int           `json:"offset"`
}

type TableSchema struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
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
	return driver.TablePage{
		Namespace: in.Namespace.toDriver(),
		Table:     in.Table,
		Limit:     driverRows(in.Limit, DefaultPageSize),
		Offset:    max(in.Offset, 0),
	}
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
		Columns: toColumnDTOs(rows.Columns),
		Rows:    rows.Rows,
		Total:   rows.Total,
		Limit:   page.Limit,
		Offset:  page.Offset,
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
