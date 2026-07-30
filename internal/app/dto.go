package app

import (
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

const (
	DefaultPageSize = 100
	MaxPageSize     = 1000

	DefaultQueryRows = 1000
	MaxQueryRows     = 10000
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

type TablePageInput struct {
	Database string `json:"database"`
	Table    string `json:"table"`
	Limit    int    `json:"limit"`
	Offset   int    `json:"offset"`
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

type QueryInput struct {
	RunID    string `json:"runId"`
	Database string `json:"database"`
	SQL      string `json:"sql"`
	MaxRows  int    `json:"maxRows"`
	Explain  bool   `json:"explain"`
}

// QueryResult carries both shapes of outcome: a statement that returns rows fills
// Columns, and one that does not leaves them empty and reports RowsAffected.
type QueryResult struct {
	Columns      []TableColumn `json:"columns"`
	Rows         [][]*string   `json:"rows"`
	RowsAffected int64         `json:"rowsAffected"`
	Truncated    bool          `json:"truncated"`
	ElapsedMs    int64         `json:"elapsedMs"`
}

type CellEditInput struct {
	Database string            `json:"database"`
	Table    string            `json:"table"`
	Column   string            `json:"column"`
	Value    *string           `json:"value"`
	Key      map[string]string `json:"key"`
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

func (in TablePageInput) toDriver() driver.TablePage {
	limit := in.Limit
	if limit <= 0 {
		limit = DefaultPageSize
	}
	if limit > MaxPageSize {
		limit = MaxPageSize
	}

	offset := in.Offset
	if offset < 0 {
		offset = 0
	}

	return driver.TablePage{
		Database: in.Database,
		Table:    in.Table,
		Limit:    limit,
		Offset:   offset,
	}
}

func (in QueryInput) toDriver() driver.Query {
	maxRows := in.MaxRows
	if maxRows <= 0 {
		maxRows = DefaultQueryRows
	}
	if maxRows > MaxQueryRows {
		maxRows = MaxQueryRows
	}

	return driver.Query{
		Database: in.Database,
		SQL:      in.SQL,
		MaxRows:  maxRows,
		Explain:  in.Explain,
	}
}

func (in CellEditInput) toDriver() driver.CellEdit {
	return driver.CellEdit{
		Database: in.Database,
		Table:    in.Table,
		Column:   in.Column,
		Value:    in.Value,
		Key:      in.Key,
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
		Columns:      toColumnDTOs(result.Columns),
		Rows:         result.Rows,
		RowsAffected: result.RowsAffected,
		Truncated:    result.Truncated,
		ElapsedMs:    elapsed.Milliseconds(),
	}
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

func (in ProfileInput) toDomain() domain.ConnectionProfile {
	port := in.Port
	if port == 0 {
		port = domain.DefaultMySQLPort
	}

	tlsMode := domain.TLSMode(in.TLSMode)
	if tlsMode == "" {
		tlsMode = domain.TLSPreferred
	}

	return domain.ConnectionProfile{
		ID:           in.ID,
		Name:         in.Name,
		Engine:       domain.EngineMySQL,
		Host:         in.Host,
		Port:         port,
		User:         in.User,
		Database:     in.Database,
		TLSMode:      tlsMode,
		SavePassword: in.SavePassword,
	}
}
