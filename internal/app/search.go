package app

import (
	"context"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// searchTimeout covers a **whole** Find in Schema rather than one read of it: a schema of two
// hundred tables is two hundred reads, and a deadline each of them would have passed comfortably
// adds up to a wait nobody agreed to. It is long because the reader can stop it — see StopSearch —
// which is the only honest bound on a read whose length is the schema's rather than this side's.
const searchTimeout = 10 * time.Minute

// SearchInput is one Find in Schema: what to look for, which half of an object to look in, and how
// to match it. The namespace is where it looks — every table of one namespace, which is what the
// tree offers it on.
//
// RunID is minted by the caller for the reason a table read's is: the call only returns once every
// table has been read, and Stop has to work before that.
type SearchInput struct {
	Namespace
	RunID           string `json:"runId"`
	Text            string `json:"text"`
	Scope           string `json:"scope"`
	Mode            string `json:"mode"`
	CaseInsensitive bool   `json:"caseInsensitive"`
}

// SearchHit is one object the search matched, and how many of its rows — or of the names it declares
// — did. It carries a count rather than the matches themselves because that is what the reader acts
// on: a table with four thousand matching rows is not a list anybody scrolls, and opening it is what
// comes next.
//
// `statement` is how they open it: the read that returns exactly the rows counted, spelled by the
// engine (`driver.RowMatches`) from the **same** condition the count was made through — so the query
// tab it lands in cannot show a different set of rows from the number that led the reader there. A
// structure hit carries none: matching names is not a read.
type SearchHit struct {
	Table     string `json:"table"`
	Matches   int64  `json:"matches"`
	Statement string `json:"statement"`
}

// SearchSkip is one object the search could not read, carrying the server's own wording. Reporting
// it and carrying on is the useful answer: a schema nearly always holds something this account may
// not read, and stopping at the first one would make the whole feature unusable there. It is the
// same bargain a script's messages strike — see StatementOutcome.
type SearchSkip struct {
	Table string `json:"table"`
	Error string `json:"error"`
}

// SearchResult is a whole Find in Schema. `searched` is how many objects were read, so the line
// under the results can say one *of* forty rather than one on its own.
type SearchResult struct {
	Hits     []SearchHit  `json:"hits"`
	Searched int          `json:"searched"`
	Skipped  []SearchSkip `json:"skipped"`
}

// FindInSchema counts, per table of a namespace, what holds the reader's text.
//
// **The two halves are different kinds of question and are answered in different places.** Which
// *rows* hold the text is a statement, so it is the driver's (`driver.Session.MatchingRows`), and
// every column of any type is read as text to answer it. Which *names* hold it is not a server's
// question at all: a field, an index and a constraint are names the catalogue reads already answer
// with, so a structure search is one read of the shape and then `domain.TextMatch` — the **same**
// rule the driver spells, which is what stops Contains meaning two things.
//
// A table nobody may read is one row missing from the answer and not the end of the search; a
// deadline or a Stop is the end of it, because every table after it would fail the same way.
func (a *ConnectionAPI) FindInSchema(sessionID string, in SearchInput) (SearchResult, error) {
	match := in.toDomain()
	if err := match.Validate(); err != nil {
		return SearchResult{}, wrapError(err)
	}

	scope := domain.SearchScope(in.Scope)
	if !scope.Known() {
		return SearchResult{}, wrapError(domain.ValidationError{
			Field:   "scope",
			Message: "is not a half of an object this app searches",
		})
	}

	schema, err := a.sessions.Schema(sessionID)
	if err != nil {
		return SearchResult{}, wrapError(err)
	}

	// Compiled once for the whole search rather than once per table, and **only for the half that
	// matches here**: a data search's expression is the *server's* to read, and refusing one this
	// side could not parse would refuse a reader their own engine's flavour of one — a backreference,
	// a lookahead — which Go has not got.
	matcher := domain.Matcher{}
	if scope == domain.SearchStructure {
		matcher, err = match.Matcher()
		if err != nil {
			return SearchResult{}, wrapError(err)
		}
	}

	ctx, release := a.runs.begin(a.ctx, in.RunID, searchTimeout)
	defer release()

	ns := in.Namespace.toDriver()
	objects, err := a.sessions.Objects(ctx, sessionID, ns)
	if err != nil {
		return SearchResult{}, wrapError(err)
	}

	found := SearchResult{Hits: []SearchHit{}, Skipped: []SearchSkip{}}
	for _, table := range objects.Tables {
		hit, err := a.searchTable(ctx, searchOne{
			sessionID: sessionID,
			schema:    schema,
			namespace: ns,
			table:     table,
			scope:     scope,
			match:     match,
			matcher:   matcher,
		})
		if err != nil {
			if ctx.Err() != nil {
				return SearchResult{}, wrapError(err)
			}
			found.Skipped = append(found.Skipped, SearchSkip{Table: table, Error: err.Error()})
			continue
		}

		found.Searched++
		if hit.Matches > 0 {
			found.Hits = append(found.Hits, hit)
		}
	}

	return found, nil
}

// StopSearch stops the search with this id. An unknown id is ignored, the same contract
// StopTableRows has: the search may have finished between the click and this call.
func (a *ConnectionAPI) StopSearch(runID string) {
	a.runs.cancel(runID)
}

// searchOne is one table of a search, which is seven things and would otherwise be seven arguments.
type searchOne struct {
	sessionID string
	schema    driver.Schema
	namespace driver.Namespace
	table     string
	scope     domain.SearchScope
	match     domain.TextMatch
	matcher   domain.Matcher
}

func (a *ConnectionAPI) searchTable(ctx context.Context, one searchOne) (SearchHit, error) {
	if one.scope == domain.SearchData {
		matches, err := a.sessions.MatchingRows(ctx, one.sessionID, driver.RowSearch{
			Namespace: one.namespace,
			Table:     one.table,
			Match:     one.match,
		})
		if err != nil {
			return SearchHit{}, err
		}
		return SearchHit{
			Table:     one.table,
			Matches:   matches.Count,
			Statement: matches.Statement,
		}, nil
	}

	shape, err := one.schema.TableStructure(ctx, one.namespace, one.table)
	if err != nil {
		return SearchHit{}, err
	}
	return SearchHit{Table: one.table, Matches: matchedNames(shape, one.table, one.matcher)}, nil
}

// matchedNames is what a structure search found about one table: its own name, and then every field,
// index and constraint whose name holds the text. It counts **names** and nothing else — a type, a
// default and an access method are the engine's own wording, and a reader looking for `khanh` is
// looking for something *called* that.
func matchedNames(shape driver.TableStructure, table string, matcher domain.Matcher) int64 {
	matched := int64(0)
	if matcher.Matches(table) {
		matched++
	}
	for _, column := range shape.Columns {
		if matcher.Matches(column.Name) {
			matched++
		}
	}
	for _, index := range shape.Indexes {
		if matcher.Matches(index.Name) {
			matched++
		}
	}
	for _, key := range shape.ForeignKeys {
		if matcher.Matches(key.Name) {
			matched++
		}
	}
	return matched
}

func (in SearchInput) toDomain() domain.TextMatch {
	return domain.TextMatch{
		Text:            in.Text,
		Mode:            domain.SearchMode(in.Mode),
		CaseInsensitive: in.CaseInsensitive,
	}
}
