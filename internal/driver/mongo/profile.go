package mongo

import (
	"context"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	mgo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// What a MongoDB server has already run **slowly**. Nothing here touches `sqlkit.Profile`: there is
// no statement to spell, every one of these being a command whose arguments are values in a document
// — which is why nothing in this file quotes anything on its way to the server.
//
// **This is the engine the word profiling comes from.** `db.setProfilingLevel(1, 100)` tells one
// database to write down every operation slower than 100ms, and it writes them into a collection of
// that database — `system.profile`, a capped one, so the server bounds what it keeps rather than
// filling a disk.
//
// **It is per database and never per server**, which is the fact that shapes this whole file: a
// reader who turned the recording on while looking at `shop` has turned it on for `shop`, and the
// pane read against `admin` will hold nothing. The namespace therefore reaches every method here,
// and the dialog says which database it is about.
//
// **What it keeps is the command as it ran**, filter values and all — the one engine where that is
// true of a *summary* rather than of a live read. The command is drawn whole in its own cell, so the
// reader reads it where it is rather than reconstructing it from the columns beside it.

// profileCollection is where a MongoDB server writes what it recorded. It is a system collection, so
// it is not in the tree's collection list — see `collections`, which drops the whole prefix.
const profileCollection = "system.profile"

// slowQueryLimit is what a request with no limit of its own reads.
const slowQueryLimit = 200

// profilingSlow is the level that records **slow operations only**, which is the one this app turns
// on. Level 2 records every operation there is, which on a busy database is a way of making it
// slower, and is not a thing to reach by pressing a switch labelled on.
const profilingSlow = 1

// SlowQueries reads `system.profile`, slowest first. The command is drawn whole into its own cell,
// in the shell's own notation like every other document this adapter shows, so what the reader sees
// is what they would have typed.
func (s *session) SlowQueries(
	ctx context.Context,
	ns driver.Namespace,
	limit int,
) (driver.SlowQueries, error) {
	if limit <= 0 {
		limit = slowQueryLimit
	}

	found, err := s.collection(ns, profileCollection).Find(
		ctx,
		bson.D{},
		options.Find().SetSort(bson.D{{Key: "millis", Value: -1}}).SetLimit(int64(limit)),
	)
	if err != nil {
		return driver.SlowQueries{}, translateError(err)
	}
	defer found.Close(ctx)

	return s.readProfile(ctx, found)
}

// readProfile turns the documents into the grid's rows. The command is drawn by the same `cell` every
// other document in this adapter is drawn by, so what the reader sees in the panel under the grid is
// the notation they would have typed.
func (s *session) readProfile(ctx context.Context, found *mgo.Cursor) (driver.SlowQueries, error) {
	list := driver.SlowQueries{
		Columns: []driver.Column{
			{Name: "When", Type: "string", FullType: "string"},
			{Name: "Duration ms", Type: "integer", FullType: "integer"},
			{Name: "Operation", Type: "string", FullType: "string"},
			{Name: "Namespace", Type: "string", FullType: "string"},
			{Name: "Docs Examined", Type: "integer", FullType: "integer"},
			{Name: "Keys Examined", Type: "integer", FullType: "integer"},
			{Name: "Plan", Type: "string", FullType: "string"},
			{Name: "Command", Type: "document", FullType: "document"},
		},
		Rows: [][]*string{},
		Refs: []driver.SlowQueryRef{},
	}

	for found.Next(ctx) {
		one := bson.Raw(found.Current)
		command := one.Lookup("command")
		namespace := one.Lookup("ns")

		list.Rows = append(list.Rows, []*string{
			cell(one.Lookup("ts")),
			cell(one.Lookup("millis")),
			cell(one.Lookup("op")),
			cell(namespace),
			cell(one.Lookup("docsExamined")),
			cell(one.Lookup("keysExamined")),
			cell(one.Lookup("planSummary")),
			cell(command),
		})
		list.Refs = append(list.Refs, driver.SlowQueryRef{
			Runnable: runnableCommand(namespace, command),
			// A profile records the command as it ran, values and all — there is nothing to fill in.
			Complete: true,
		})
	}
	if err := found.Err(); err != nil {
		return driver.SlowQueries{}, translateError(err)
	}
	return list, nil
}

// runnableCommand writes a recorded operation out as something that would **run somewhere else** —
// in `mongosh`, or in a script. It is `Dialect.Runnable`'s counterpart, and it is here rather than
// there for the reason nothing in this adapter touches `sqlkit`: a command is a document, and the
// database it belongs to is named by a *method call* rather than by a statement.
//
// `getSiblingDB` is what names it. A shell's `db` is whichever database the reader is pointed at, so
// a command pasted without one would run against theirs rather than against the one it was recorded
// in — and `system.profile` is per database precisely because that distinction matters here.
//
// A row whose command the server did not record answers with nothing.
func runnableCommand(namespace, command bson.RawValue) string {
	if _, ok := command.DocumentOK(); !ok {
		return ""
	}

	// A profile's `ns` is `database.collection`, and a dot inside a collection name is legal while
	// one inside a database name is not — so the first dot is where the two part.
	text, _ := namespace.StringValueOK()
	database, _, _ := strings.Cut(text, ".")
	if database == "" {
		return "db.runCommand(" + valueText(command) + ")"
	}
	return "db.getSiblingDB(" + valueText(database) + ").runCommand(" + valueText(command) + ")"
}

// ProfileConfig asks this database what level it is at. `profile: -1` is the read — the same command
// as the write, with the level that means "tell me rather than set me", which is this engine's own
// arrangement rather than a trick.
func (s *session) ProfileConfig(
	ctx context.Context,
	ns driver.Namespace,
) (driver.ProfileConfig, error) {
	raw, err := s.runCommand(ctx, ns, bson.D{{Key: "profile", Value: -1}})
	if err != nil {
		return driver.ProfileConfig{}, err
	}

	level := numberField(raw, "was")
	slowMs := numberField(raw, "slowms")

	return driver.ProfileConfig{
		Enabled:      level > 0,
		ThresholdMs:  slowMs,
		Configurable: true,
		Ready:        true,
		Detail: []driver.ProfileFact{
			{Label: "Database", Value: s.databaseName(ns)},
			{Label: "Level", Value: strconv.Itoa(level) + " — " + levelText(level)},
			{Label: "Written to", Value: profileCollection},
		},
	}, nil
}

// levelText is what a level means, in the server's own three answers. It is spelled out because
// `1` and `2` are a world apart on a busy database and neither number says so.
func levelText(level int) string {
	switch level {
	case profilingSlow:
		return "slow operations only"
	case 2:
		return "every operation"
	default:
		return "off"
	}
}

// ProfileStatements spells the changes as the commands a reader would type into the tab beside this
// one — `ProcessStatements`' arrangement, and here it is again the whole of what a confirmation
// shows.
//
// The database is not in them. A shell's `db` is whichever database it is pointed at, which is the
// namespace this pane is being read in, and writing `db.getSiblingDB(…)` in front of every line
// would be spelling this app's arrangement rather than the reader's command.
func (*session) ProfileStatements(changes []driver.ProfileChange) []string {
	statements := make([]string, 0, len(changes))
	for _, change := range changes {
		switch change.Action {
		case domain.ProfileEnable:
			statements = append(statements, "db.setProfilingLevel("+
				strconv.Itoa(profilingSlow)+", { slowms: "+
				strconv.Itoa(change.ThresholdMs)+" })")
		case domain.ProfileDisable:
			statements = append(statements, "db.setProfilingLevel(0)")
		case domain.ProfileReset:
			statements = append(statements, "db."+profileCollection+".drop()")
		}
	}
	return statements
}

// ApplyProfileChanges runs exactly what ProfileStatements showed, in that order, stopping at the
// first failure.
//
// **Dropping the collection is how it is emptied**, and it is the server's own way: `system.profile`
// is capped, so its documents cannot be deleted one by one, and the server recreates it the next time
// it records something. The recording has to be off while it happens — a server will not drop the
// collection it is writing to — so that is done here rather than left to the reader as a step they
// would find out about from a refusal.
func (s *session) ApplyProfileChanges(
	ctx context.Context,
	ns driver.Namespace,
	changes []driver.ProfileChange,
) error {
	for _, change := range changes {
		if err := s.applyProfile(ctx, ns, change); err != nil {
			return err
		}
	}
	return nil
}

func (s *session) applyProfile(
	ctx context.Context,
	ns driver.Namespace,
	change driver.ProfileChange,
) error {
	switch change.Action {
	case domain.ProfileEnable:
		if change.ThresholdMs < 0 {
			return domain.ValidationError{
				Field:   "threshold",
				Message: "is not a number of milliseconds",
			}
		}
		_, err := s.runCommand(ctx, ns, bson.D{
			{Key: "profile", Value: profilingSlow},
			{Key: "slowms", Value: change.ThresholdMs},
		})
		return err

	case domain.ProfileDisable:
		_, err := s.runCommand(ctx, ns, bson.D{{Key: "profile", Value: 0}})
		return err

	case domain.ProfileReset:
		return s.emptyProfile(ctx, ns)

	default:
		return domain.ValidationError{Field: "action", Message: "is not a change to profiling"}
	}
}

// emptyProfile turns the recording off, drops the capped collection, and puts the level back where
// it was. The level is read first so that a reader who was recording is recording afterwards: a
// reset that quietly turned their profiling off would be a reset that lost them the next hour.
func (s *session) emptyProfile(ctx context.Context, ns driver.Namespace) error {
	was, err := s.ProfileConfig(ctx, ns)
	if err != nil {
		return err
	}

	if _, err := s.runCommand(ctx, ns, bson.D{{Key: "profile", Value: 0}}); err != nil {
		return err
	}
	if err := s.collection(ns, profileCollection).Drop(ctx); err != nil {
		return translateError(err)
	}
	if !was.Enabled {
		return nil
	}

	_, err = s.runCommand(ctx, ns, bson.D{
		{Key: "profile", Value: profilingSlow},
		{Key: "slowms", Value: was.ThresholdMs},
	})
	return err
}

// numberField is one numeric field of a document, or zero for anything else. It is for the two facts
// a config is read from rather than for a cell.
func numberField(raw bson.Raw, name string) int {
	value, err := raw.LookupErr(name)
	if err != nil {
		return 0
	}
	number, ok := value.AsInt64OK()
	if !ok {
		return 0
	}
	return int(number)
}
