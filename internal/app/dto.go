package app

import (
	"time"

	"github.com/khanhsnd/vibe-db/internal/compare"
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

// TLSSettings is the transport half of a profile on the wire. Every path is a path on the
// reader's own machine and is only ever read there — nothing in it crosses to a server.
type TLSSettings struct {
	Mode       string `json:"mode"`
	RootCert   string `json:"rootCert"`
	ClientCert string `json:"clientCert"`
	ClientKey  string `json:"clientKey"`
	ServerName string `json:"serverName"`
}

// SSHSettings is the bastion half. The pass phrase or password it needs is *not* here: it is a
// secret and travels in ProfileInput.SSHSecret on its way in, and never comes back out.
type SSHSettings struct {
	Enabled       bool   `json:"enabled"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	User          string `json:"user"`
	Auth          string `json:"auth"`
	KeyPath       string `json:"keyPath"`
	KnownHosts    string `json:"knownHosts"`
	StrictHostKey bool   `json:"strictHostKey"`
}

type Profile struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Engine      string `json:"engine"`
	Environment string `json:"environment"`
	// Colour is the tint the sidebar draws this connection and everything under it in. It is the
	// one field of a profile no server hears about — see domain.Colour.
	Colour   string `json:"colour"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Database string `json:"database"`
	Auth     string `json:"auth"`
	Region   string `json:"region"`

	TLS TLSSettings `json:"tls"`
	SSH SSHSettings `json:"ssh"`

	SavePassword bool `json:"savePassword"`
	HasPassword  bool `json:"hasPassword"`
	// HasSSHSecret is the same fact about the bastion's own secret, so the dialog can say a pass
	// phrase is already stored rather than making the reader type one they have saved.
	HasSSHSecret bool `json:"hasSshSecret"`
}

type ProfileInput struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Engine      string `json:"engine"`
	Environment string `json:"environment"`
	Colour      string `json:"colour"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	User        string `json:"user"`
	Password    string `json:"password"`
	Database    string `json:"database"`
	Auth        string `json:"auth"`
	Region      string `json:"region"`

	TLS TLSSettings `json:"tls"`
	SSH SSHSettings `json:"ssh"`
	// SSHSecret is the bastion's password, or the pass phrase of its private key. Blank means
	// "keep whatever is stored", exactly as a blank Password does.
	SSHSecret string `json:"sshSecret"`

	SavePassword bool `json:"savePassword"`
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
	Name string `json:"name"`
	// FirstCollection is the collection a new MongoDB database is made with, a document store
	// having no way to hold an empty one. Every other engine leaves it blank and ignores it.
	FirstCollection  string   `json:"firstCollection"`
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
	// Plan is filled only by a run that asked to be explained, and is null for every other. Explain
	// runs the statement like any other and asks about it as well, so the Rows above are the
	// statement's own and the plan is *about* that run rather than instead of it.
	Plan *QueryPlan `json:"plan"`
}

// PlanNode is one step of a plan. `depth` is how deep in the plan's tree it sits and the steps
// arrive in the order the tree reads, so the list is the whole tree — which is what the grid drawing
// it needs, a grid having rows and not branches.
type PlanNode struct {
	Type     string  `json:"type"`
	Detail   string  `json:"detail"`
	Depth    int     `json:"depth"`
	Rows     float64 `json:"rows"`
	Cost     float64 `json:"cost"`
	SelfCost float64 `json:"selfCost"`
}

// PlanFact is one thing the engine said about the plan as a whole, in its own wording. Pairs rather
// than fields because what a plan reports is the engine's own — see driver.PlanFact.
type PlanFact struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// QueryPlan is a plan in the forms the reader reads it in, and the first is the engine's own:
// `columns` and `rows` are what an EXPLAIN typed by hand answers with, and everything else is read
// out of that same answer. `costed` says the engine puts numbers on a step at all — a cost, and a row
// estimate — so a plan from an engine that does not is drawn without those columns rather than with
// columns of zeroes.
type QueryPlan struct {
	Columns []TableColumn `json:"columns"`
	Rows    [][]*string   `json:"rows"`
	Nodes   []PlanNode    `json:"nodes"`
	Summary []PlanFact    `json:"summary"`
	JSON    string        `json:"json"`
	Costed  bool          `json:"costed"`
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

// StatementReview is one statement of a script as the reviewer reads it, without running any of
// it. `start` and `end` are byte offsets into the script the editor holds, so a warning marks the
// statement that earned it rather than repeating it.
//
// `confirm` is the whole point of the shape: it is the *answer* to "does this have to be asked
// about here", worked out from the hazard and the connection's environment on the Go side, so the
// editor never re-derives a rule about what is dangerous — see domain.Environment.
type StatementReview struct {
	Start   int    `json:"start"`
	End     int    `json:"end"`
	SQL     string `json:"sql"`
	Keyword string `json:"keyword"`
	Hazard  string `json:"hazard"`
	Confirm bool   `json:"confirm"`
}

// ScriptProblem is something wrong with a script that could be proved without a server.
type ScriptProblem struct {
	Start   int    `json:"start"`
	End     int    `json:"end"`
	Message string `json:"message"`
}

// ScriptReview is everything a script says about itself before anything runs it. `environment` is
// the connection's, so the editor can *say* which one it is about to run against without asking
// a second question.
type ScriptReview struct {
	Statements  []StatementReview `json:"statements"`
	Problems    []ScriptProblem   `json:"problems"`
	Environment string            `json:"environment"`
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

// A comparison's four statuses, spelled on the wire exactly as compare.Status spells them. They
// are a string rather than a flag per kind because one word answers for a field, an index, a
// constraint and a row alike, and the grid draws one colour per word.
//
// `added` is on the right and not on the left; `removed` is the other way about.

// FieldDiff is one field of a structure comparison: whichever sides hold it, and what differs.
// `left` or `right` is null for a field the other side alone holds, which is what the blank
// opposite an added or a removed one is drawn from.
type FieldDiff struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	// Differs names the facts that differ — `type`, `nullable`, `default`, `primary key` — so the
	// reader is told *which* fact changed rather than only that one did.
	Differs []string     `json:"differs"`
	Left    *TableColumn `json:"left"`
	Right   *TableColumn `json:"right"`
}

type IndexDiff struct {
	Name    string     `json:"name"`
	Status  string     `json:"status"`
	Differs []string   `json:"differs"`
	Left    *IndexInfo `json:"left"`
	Right   *IndexInfo `json:"right"`
}

type ForeignKeyDiff struct {
	Name    string          `json:"name"`
	Status  string          `json:"status"`
	Differs []string        `json:"differs"`
	Left    *ForeignKeyInfo `json:"left"`
	Right   *ForeignKeyInfo `json:"right"`
}

// StructureDiff is two tables' shapes lined up, one entry per thing either side holds and in the
// left's own reading order — so row *n* of the left pane and row *n* of the right pane are the
// same field, which is the whole of what makes two panes readable side by side.
type StructureDiff struct {
	Fields      []FieldDiff      `json:"fields"`
	Indexes     []IndexDiff      `json:"indexes"`
	ForeignKeys []ForeignKeyDiff `json:"foreignKeys"`
}

// RowDiff is one row of a data comparison. `left` and `right` hold one value per compared column,
// in the order DataDiff.columns names them, and one of the two is null for a row only the other
// side has.
type RowDiff struct {
	Status string    `json:"status"`
	Key    []string  `json:"key"`
	Left   []*string `json:"left"`
	Right  []*string `json:"right"`
	// Differs is one flag per compared column, true where the sides hold different values, and is
	// empty for a row only one side holds — there being nothing to have compared it against.
	Differs []bool `json:"differs"`
}

// DataDiff is two tables' rows lined up.
type DataDiff struct {
	// Key is the columns rows were matched on, and `keyDeclared` says they are the tables' own
	// primary key rather than the fallback. On the fallback a row is matched on the whole of
	// itself, so nothing can be `changed` — and the reader has to be told that rather than shown
	// "no changed rows" as though it meant there were none.
	Key         []string `json:"key"`
	KeyDeclared bool     `json:"keyDeclared"`
	// Columns is the columns both sides hold, in the left's order, and the only ones compared.
	// They arrive as the left's own columns rather than as names, because the grid draws a type.
	Columns []TableColumn `json:"columns"`
	// The columns one side holds alone, named rather than compared: a value with nothing opposite
	// it is a difference between two *shapes*, which the structure comparison is where to read.
	LeftOnly  []string  `json:"leftOnly"`
	RightOnly []string  `json:"rightOnly"`
	Rows      []RowDiff `json:"rows"`
	// What each side's read returned, and whether the table held more. A comparison is bounded on
	// purpose — see DefaultCompareRows — so it says which side it did not see the whole of.
	LeftRows       int  `json:"leftRows"`
	RightRows      int  `json:"rightRows"`
	LeftTruncated  bool `json:"leftTruncated"`
	RightTruncated bool `json:"rightTruncated"`
}

// TableChangeInput is one change to a table's shape as a migration declares it: `kind` says which
// of the seven it is and only the field belonging to that kind is read. `name` is what the change
// is addressed to — the column, index or constraint — and is empty for one that only adds.
//
// It is the *same* list for both halves of the round trip: PreviewTableChanges spells it and
// ApplyTableChanges runs it, so what the reader was shown and what happened cannot be two things.
type TableChangeInput struct {
	Kind   string          `json:"kind"`
	Name   string          `json:"name"`
	Column ColumnInput     `json:"column"`
	Index  IndexInput      `json:"index"`
	Key    ForeignKeyInput `json:"key"`
}

// RowWriteInput is a batch of rows and what they are to do to a table — the data half of a
// migration, and the same shape an import already travels as. `mode` is a token from a closed
// list, and `keys` names the columns that identify a stored row, which every mode but an append
// needs: without one there is nothing to change or remove.
type RowWriteInput struct {
	Namespace
	Table   string      `json:"table"`
	Mode    string      `json:"mode"`
	Columns []string    `json:"columns"`
	Keys    []string    `json:"keys"`
	Rows    [][]*string `json:"rows"`
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

func toProfileDTO(profile domain.ConnectionProfile, hasPassword, hasSSHSecret bool) Profile {
	return Profile{
		ID:          profile.ID,
		Name:        profile.Name,
		Engine:      string(profile.Engine),
		Environment: string(profile.Environment),
		Colour:      string(profile.Colour),
		Host:        profile.Host,
		Port:        profile.Port,
		User:        profile.User,
		Database:    profile.Database,
		Auth:        string(profile.Auth),
		Region:      profile.Region,
		TLS: TLSSettings{
			Mode:       string(profile.TLS.Mode),
			RootCert:   profile.TLS.RootCert,
			ClientCert: profile.TLS.ClientCert,
			ClientKey:  profile.TLS.ClientKey,
			ServerName: profile.TLS.ServerName,
		},
		SSH: SSHSettings{
			Enabled:       profile.SSH.Enabled,
			Host:          profile.SSH.Host,
			Port:          profile.SSH.Port,
			User:          profile.SSH.User,
			Auth:          string(profile.SSH.Auth),
			KeyPath:       profile.SSH.KeyPath,
			KnownHosts:    profile.SSH.KnownHosts,
			StrictHostKey: profile.SSH.StrictHostKey,
		},
		SavePassword: profile.SavePassword,
		HasPassword:  hasPassword,
		HasSSHSecret: hasSSHSecret,
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
		Plan:         toPlanDTO(result.Plan),
	}
}

// toPlanDTO carries a plan across as it was read. A run that asked for no plan, or an engine that
// could not give one, has none — and the absence is the answer, not an empty plan.
func toPlanDTO(plan *driver.Plan) *QueryPlan {
	if plan == nil {
		return nil
	}

	nodes := make([]PlanNode, 0, len(plan.Nodes))
	for _, node := range plan.Nodes {
		nodes = append(nodes, PlanNode{
			Type:     node.Type,
			Detail:   node.Detail,
			Depth:    node.Depth,
			Rows:     node.Rows,
			Cost:     node.Cost,
			SelfCost: node.SelfCost,
		})
	}

	facts := make([]PlanFact, 0, len(plan.Summary))
	for _, fact := range plan.Summary {
		facts = append(facts, PlanFact{Label: fact.Label, Value: fact.Value})
	}

	return &QueryPlan{
		Columns: toColumnDTOs(plan.Columns),
		Rows:    plan.Rows,
		Nodes:   nodes,
		Summary: facts,
		JSON:    plan.JSON,
		Costed:  plan.Costed,
	}
}

// toReviewDTO pairs what a script will do with where it will do it, which is the one place the
// two meet: domain.Environment decides, and this carries the decision across the wire so the
// editor has a `confirm` to act on rather than a rule to re-implement.
func toReviewDTO(review driver.Review, environment domain.Environment) ScriptReview {
	statements := make([]StatementReview, 0, len(review.Statements))
	for _, statement := range review.Statements {
		statements = append(statements, StatementReview{
			Start:   statement.Start,
			End:     statement.End,
			SQL:     statement.SQL,
			Keyword: statement.Keyword,
			Hazard:  string(statement.Hazard),
			Confirm: environment.NeedsConfirmation(statement.Hazard),
		})
	}

	problems := make([]ScriptProblem, 0, len(review.Problems))
	for _, problem := range review.Problems {
		problems = append(problems, ScriptProblem{
			Start:   problem.Start,
			End:     problem.End,
			Message: problem.Message,
		})
	}

	return ScriptReview{
		Statements:  statements,
		Problems:    problems,
		Environment: string(environment),
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

// toDriver reads a change the migration declared. A kind this side does not know is carried across
// as it stands: the driver spells nothing for it and the run refuses it, so one unknown word is
// answered in one place rather than guessed at in two.
func (in TableChangeInput) toDriver() driver.TableChange {
	return driver.TableChange{
		Kind:   driver.TableChangeKind(in.Kind),
		Name:   in.Name,
		Column: in.Column.toDriver(),
		Index:  in.Index.toDriver(),
		Key:    in.Key.toDriver(),
	}
}

func toDriverChanges(in []TableChangeInput) []driver.TableChange {
	changes := make([]driver.TableChange, 0, len(in))
	for _, change := range in {
		changes = append(changes, change.toDriver())
	}
	return changes
}

// toDriver fixes the batch this side rather than taking the caller's: a migration's rows are the
// ones it just showed, and how many a server will accept is a fact about the server.
func (in RowWriteInput) toDriver() driver.RowWrite {
	return driver.RowWrite{
		Mode:    driver.RowWriteMode(in.Mode),
		Columns: in.Columns,
		Keys:    in.Keys,
		Rows:    in.Rows,
		Batch:   MigrationBatch,
	}
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
		FirstCollection:  in.FirstCollection,
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

// toDatabaseOptionsDTO answers with an empty list rather than a null one for every list the engine
// has no such concept of. A nil slice crosses the wire as `null`, and the dialog narrows one list by
// another — so a list nobody filled has to arrive as a list nobody filled and not as an absence.
func toDatabaseOptionsDTO(source driver.DatabaseOptions) DatabaseOptions {
	return DatabaseOptions{
		Owners:      names(source.Owners),
		Templates:   names(source.Templates),
		Encodings:   names(source.Encodings),
		Collations:  names(source.Collations),
		Tablespaces: names(source.Tablespaces),
		Extensions:  names(source.Extensions),
	}
}

func names(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
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
		indexes = append(indexes, toIndexDTO(index))
	}

	keys := make([]ForeignKeyInfo, 0, len(source.ForeignKeys))
	for _, key := range source.ForeignKeys {
		keys = append(keys, toForeignKeyDTO(key))
	}

	return TableStructure{
		Columns:     toColumnDTOs(source.Columns),
		Indexes:     indexes,
		ForeignKeys: keys,
	}
}

func toIndexDTO(index driver.Index) IndexInfo {
	return IndexInfo{
		Name:    index.Name,
		Columns: index.Columns,
		Unique:  index.Unique,
		Primary: index.Primary,
		Method:  index.Method,
	}
}

func toForeignKeyDTO(key driver.ForeignKey) ForeignKeyInfo {
	return ForeignKeyInfo{
		Name:              key.Name,
		Columns:           key.Columns,
		ReferencedSchema:  key.ReferencedSchema,
		ReferencedTable:   key.ReferencedTable,
		ReferencedColumns: key.ReferencedColumns,
		OnUpdate:          key.OnUpdate,
		OnDelete:          key.OnDelete,
	}
}

// toStructureDiffDTO carries a shape comparison across as it was worked out. A side that has not
// got the thing arrives as `null` rather than as an empty object, because a field with no type is
// not the same fact as a field that is not there at all.
func toStructureDiffDTO(source compare.Structure) StructureDiff {
	fields := make([]FieldDiff, 0, len(source.Fields))
	for _, pair := range source.Fields {
		fields = append(fields, FieldDiff{
			Name:    pair.Name,
			Status:  string(pair.Status),
			Differs: names(pair.Differs),
			Left:    side(pair.Left, toColumnDTO),
			Right:   side(pair.Right, toColumnDTO),
		})
	}

	indexes := make([]IndexDiff, 0, len(source.Indexes))
	for _, pair := range source.Indexes {
		indexes = append(indexes, IndexDiff{
			Name:    pair.Name,
			Status:  string(pair.Status),
			Differs: names(pair.Differs),
			Left:    side(pair.Left, toIndexDTO),
			Right:   side(pair.Right, toIndexDTO),
		})
	}

	keys := make([]ForeignKeyDiff, 0, len(source.ForeignKeys))
	for _, pair := range source.ForeignKeys {
		keys = append(keys, ForeignKeyDiff{
			Name:    pair.Name,
			Status:  string(pair.Status),
			Differs: names(pair.Differs),
			Left:    side(pair.Left, toForeignKeyDTO),
			Right:   side(pair.Right, toForeignKeyDTO),
		})
	}

	return StructureDiff{Fields: fields, Indexes: indexes, ForeignKeys: keys}
}

// toDataDiffDTO carries a row comparison across, and takes the compared columns' own declarations
// from the side they were read in the order of — the left — so the grid drawing them has a type to
// draw and a key to mark.
func toDataDiffDTO(source compare.Data, left []driver.Column) DataDiff {
	columns := make([]TableColumn, 0, len(source.Columns))
	for _, name := range source.Columns {
		for _, column := range left {
			if column.Name == name {
				columns = append(columns, toColumnDTO(column))
				break
			}
		}
	}

	rows := make([]RowDiff, 0, len(source.Rows))
	for _, pair := range source.Rows {
		rows = append(rows, RowDiff{
			Status:  string(pair.Status),
			Key:     names(pair.Key),
			Left:    pair.Left,
			Right:   pair.Right,
			Differs: flags(pair.Differs),
		})
	}

	return DataDiff{
		Key:            names(source.Key.Columns),
		KeyDeclared:    source.Key.Declared,
		Columns:        columns,
		LeftOnly:       names(source.LeftOnly),
		RightOnly:      names(source.RightOnly),
		Rows:           rows,
		LeftRows:       source.LeftRows,
		RightRows:      source.RightRows,
		LeftTruncated:  source.LeftTruncated,
		RightTruncated: source.RightTruncated,
	}
}

// side carries one half of a comparison's pair across, and answers nil for the half that has not
// got the thing at all.
func side[From any, To any](source *From, to func(From) To) *To {
	if source == nil {
		return nil
	}
	converted := to(*source)
	return &converted
}

// flags is `names` for a list of booleans: a nil slice crosses the wire as `null`, and a caller
// indexing into what a row differs by has to find a list there.
func flags(list []bool) []bool {
	if list == nil {
		return []bool{}
	}
	return list
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
		columns = append(columns, toColumnDTO(column))
	}
	return columns
}

func toColumnDTO(column driver.Column) TableColumn {
	return TableColumn{
		Name:       column.Name,
		Type:       column.Type,
		FullType:   column.FullType,
		Nullable:   column.Nullable,
		PrimaryKey: column.PrimaryKey,
		Default:    column.Default,
	}
}

// toDomain fills in what the dialog may leave blank. An unnamed engine is MySQL, which is
// what a profile written before the second engine existed means, and the port then follows
// from the engine rather than from a constant. An unnamed authentication is a password, for the
// same reason: it is what every profile did before there was anything else to name.
func (in ProfileInput) toDomain() domain.ConnectionProfile {
	engine := domain.Engine(in.Engine)
	if engine == "" {
		engine = domain.EngineMySQL
	}

	port := in.Port
	if port == 0 {
		port = engine.DefaultPort()
	}

	tlsMode := domain.TLSMode(in.TLS.Mode)
	if tlsMode == "" {
		tlsMode = domain.TLSPreferred
	}

	auth := domain.AuthMethod(in.Auth)
	if auth == "" {
		auth = domain.AuthPassword
	}

	return domain.ConnectionProfile{
		ID:          in.ID,
		Name:        in.Name,
		Engine:      engine,
		Environment: domain.Environment(in.Environment),
		Colour:      domain.Colour(in.Colour),
		Host:        in.Host,
		Port:        port,
		User:        in.User,
		Database:    in.Database,
		Auth:        auth,
		Region:      in.Region,
		TLS: domain.TLSOptions{
			Mode:       tlsMode,
			RootCert:   in.TLS.RootCert,
			ClientCert: in.TLS.ClientCert,
			ClientKey:  in.TLS.ClientKey,
			ServerName: in.TLS.ServerName,
		},
		SSH: in.SSH.toDomain(),
		// A minted password is not one there is anything to save: it lasts minutes, and one read
		// back out of the credential store later would only fail.
		SavePassword: in.SavePassword && !auth.Minted(),
	}
}

// toDomain fills in the bastion's own default port, and reads an unnamed authentication as a key,
// which is what an SSH tunnel is nearly always set up with.
func (in SSHSettings) toDomain() domain.SSHTunnel {
	port := in.Port
	if port == 0 {
		port = domain.DefaultSSHPort
	}

	auth := domain.SSHAuth(in.Auth)
	if auth == "" {
		auth = domain.SSHKey
	}

	return domain.SSHTunnel{
		Enabled:       in.Enabled,
		Host:          in.Host,
		Port:          port,
		User:          in.User,
		Auth:          auth,
		KeyPath:       in.KeyPath,
		KnownHosts:    in.KnownHosts,
		StrictHostKey: in.StrictHostKey,
	}
}
