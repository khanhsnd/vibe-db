package app

import (
	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

const (
	DefaultPageSize = 100
	MaxPageSize     = 1000
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
	columns := make([]TableColumn, 0, len(rows.Columns))
	for _, column := range rows.Columns {
		columns = append(columns, TableColumn{
			Name:       column.Name,
			Type:       column.Type,
			FullType:   column.FullType,
			Nullable:   column.Nullable,
			PrimaryKey: column.PrimaryKey,
			Default:    column.Default,
		})
	}

	return TableRows{
		Columns: columns,
		Rows:    rows.Rows,
		Total:   rows.Total,
		Limit:   page.Limit,
		Offset:  page.Offset,
	}
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
