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
	"github.com/khanhsnd/vibe-db/internal/driver/sqlkit"
)

// Query runs one shell command. What it understands is the commands a reader actually types, and it
// is deliberately a *list* rather than a language: anything beyond it is spelled `db.runCommand({…})`
// and reaches the server verbatim, which is a better answer than a half-built JavaScript.
func (s *session) Query(ctx context.Context, query driver.Query) (driver.QueryResult, error) {
	command := sqlkit.TrimStatement(query.SQL)
	if command == "" {
		return driver.QueryResult{}, domain.ErrEmptyStatement
	}

	one, err := parse(command)
	if err != nil {
		return driver.QueryResult{}, err
	}

	// A plan is read **before** the command runs, and that order is the whole of what keeps it safe: a
	// command with no plan says so — `UNSUPPORTED` — while it has still changed nothing, and what a
	// write was going to do is not what the server would say once it has done it.
	var plan *driver.Plan
	if query.Explain {
		if plan, err = s.explainPlan(ctx, query.Namespace, one); err != nil {
			return driver.QueryResult{}, err
		}
	}

	result, err := s.execute(ctx, query, one)
	if err != nil {
		return driver.QueryResult{}, err
	}

	result.Plan = plan
	return result, nil
}

// execute runs the command against whichever of the two it addresses.
func (s *session) execute(
	ctx context.Context,
	query driver.Query,
	one statement,
) (driver.QueryResult, error) {
	if one.Collection == "" {
		return s.onDatabase(ctx, query, one)
	}
	return s.onCollection(ctx, query, one)
}

// cursorBounds is what the cursor methods chained onto a read amount to. They are read here rather
// than by the parser because only the interpreter knows which of them a given method honours.
type cursorBounds struct {
	Sort  bson.D
	Skip  int64
	Limit int64
	// Count says the chain ended in `.count()`, which turns a read of documents into a read of how
	// many there are.
	Count bool
}

func bounds(one statement) (cursorBounds, error) {
	read := cursorBounds{}

	for _, chained := range one.Chain {
		switch strings.ToLower(chained.Method) {
		case "sort":
			document, ok := chained.Args[0].(bson.D)
			if len(chained.Args) != 1 || !ok {
				return read, statementError("sort takes one document of keys")
			}
			read.Sort = document
		case "skip":
			value, err := integer(chained.Args, "skip")
			if err != nil {
				return read, err
			}
			read.Skip = value
		case "limit":
			value, err := integer(chained.Args, "limit")
			if err != nil {
				return read, err
			}
			read.Limit = value
		case "count", "size":
			read.Count = true
		// The three that change nothing about what is read, only how the shell would have printed
		// it. They are accepted so a command pasted out of a terminal runs unchanged.
		case "pretty", "toarray", "explain":
		default:
			return read, statementError(chained.Method + " is not a cursor method this side knows")
		}
	}

	return read, nil
}

// onCollection runs the commands addressed to one collection.
func (s *session) onCollection(
	ctx context.Context,
	query driver.Query,
	one statement,
) (driver.QueryResult, error) {
	collection := s.collection(query.Namespace, one.Collection)

	read, err := bounds(one)
	if err != nil {
		return driver.QueryResult{}, err
	}

	switch strings.ToLower(one.Method) {
	case "find":
		if read.Count {
			return counted(collection.CountDocuments(ctx, one.Filter()))
		}
		return s.find(ctx, query, one, read)

	case "findone":
		read.Limit = 1
		return s.find(ctx, query, one, read)

	case "aggregate":
		pipeline, err := stages(one.Arg(0))
		if err != nil {
			return driver.QueryResult{}, err
		}
		return s.cursor(ctx, query.MaxRows, func() (*mgo.Cursor, error) {
			return collection.Aggregate(ctx, pipeline)
		})

	case "countdocuments", "count":
		return counted(collection.CountDocuments(ctx, one.Filter()))

	case "estimateddocumentcount":
		return counted(collection.EstimatedDocumentCount(ctx))

	case "distinct":
		return s.distinct(ctx, query, one)

	case "getindexes":
		return s.cursor(ctx, query.MaxRows, func() (*mgo.Cursor, error) {
			return collection.Indexes().List(ctx)
		})

	case "stats":
		return s.aggregated(ctx, query.MaxRows, collection, bson.D{
			{Key: "$collStats", Value: bson.D{{Key: "storageStats", Value: bson.D{}}}},
		})

	case "insertone":
		if _, err := collection.InsertOne(ctx, one.Arg(0)); err != nil {
			return driver.QueryResult{}, translateError(err)
		}
		return affected(1), nil

	case "insertmany":
		documents, err := stages(one.Arg(0))
		if err != nil {
			return driver.QueryResult{}, err
		}
		written := make([]any, len(documents))
		for i, document := range documents {
			written[i] = document
		}
		outcome, err := collection.InsertMany(ctx, written)
		if err != nil {
			return driver.QueryResult{}, translateError(err)
		}
		return affected(int64(len(outcome.InsertedIDs))), nil

	case "updateone", "updatemany":
		return s.update(ctx, collection, one)

	case "replaceone":
		outcome, err := collection.ReplaceOne(ctx, one.Filter(), one.Arg(1))
		if err != nil {
			return driver.QueryResult{}, translateError(err)
		}
		return affected(outcome.ModifiedCount + outcome.UpsertedCount), nil

	case "deleteone":
		outcome, err := collection.DeleteOne(ctx, one.Filter())
		if err != nil {
			return driver.QueryResult{}, translateError(err)
		}
		return affected(outcome.DeletedCount), nil

	case "deletemany":
		outcome, err := collection.DeleteMany(ctx, one.Filter())
		if err != nil {
			return driver.QueryResult{}, translateError(err)
		}
		return affected(outcome.DeletedCount), nil

	case "findoneandupdate":
		return s.returning(ctx, collection.FindOneAndUpdate(ctx, one.Filter(), one.Arg(1)))

	case "findoneandreplace":
		return s.returning(ctx, collection.FindOneAndReplace(ctx, one.Filter(), one.Arg(1)))

	case "findoneanddelete":
		return s.returning(ctx, collection.FindOneAndDelete(ctx, one.Filter()))

	case "createindex":
		return s.createIndex(ctx, collection, one)

	case "dropindex":
		name, ok := oneString(one.Args)
		if !ok {
			return driver.QueryResult{}, statementError("dropIndex takes the index's name")
		}
		return affected(0), translateError(collection.Indexes().DropOne(ctx, name))

	case "dropindexes":
		return affected(0), translateError(collection.Indexes().DropAll(ctx))

	case "drop":
		return affected(0), translateError(collection.Drop(ctx))

	default:
		return driver.QueryResult{}, unknownMethod(one.Method)
	}
}

// onDatabase runs the commands addressed to the database itself. `runCommand` is the one that makes
// the rest of this list optional rather than exhaustive: whatever the server can be told to do, it
// can be told to do through that.
func (s *session) onDatabase(
	ctx context.Context,
	query driver.Query,
	one statement,
) (driver.QueryResult, error) {
	switch strings.ToLower(one.Method) {
	case "runcommand":
		raw, err := s.runCommand(ctx, query.Namespace, one.Filter())
		if err != nil {
			return driver.QueryResult{}, err
		}
		return oneDocument(raw), nil

	case "stats", "dbstats":
		raw, err := s.runCommand(ctx, query.Namespace, bson.D{{Key: "dbStats", Value: 1}})
		if err != nil {
			return driver.QueryResult{}, err
		}
		return oneDocument(raw), nil

	case "getcollectionnames":
		found, err := s.collections(ctx, query.Namespace)
		if err != nil {
			return driver.QueryResult{}, err
		}
		rows := make([][]*string, 0, len(found))
		for _, spec := range found {
			rows = append(rows, []*string{cellText(spec.Name)})
		}
		return driver.QueryResult{
			Columns: []driver.Column{{Name: "name", Type: "string", FullType: "string"}},
			Rows:    rows,
		}, nil

	case "aggregate":
		pipeline, err := stages(one.Arg(0))
		if err != nil {
			return driver.QueryResult{}, err
		}
		return s.cursor(ctx, query.MaxRows, func() (*mgo.Cursor, error) {
			return s.database(query.Namespace).Aggregate(ctx, pipeline)
		})

	case "createcollection":
		name, ok := oneString(one.Args[:min(1, len(one.Args))])
		if !ok {
			return driver.QueryResult{}, statementError("createCollection takes the collection's name")
		}
		err := s.database(query.Namespace).CreateCollection(ctx, name)
		return affected(0), translateError(err)

	case "createview":
		return s.createView(ctx, query.Namespace, one)

	case "dropdatabase":
		err := s.database(query.Namespace).Drop(ctx)
		return affected(0), translateError(err)

	default:
		return driver.QueryResult{}, unknownMethod(one.Method)
	}
}

// find is the read a reader writes most, and the one whose rows can be written back: its documents
// map one-to-one onto stored ones, and `_id` is what addresses them. A projection that leaves `_id`
// out is read-only, which `sqlkit.MergeSchema` is what decides — the same test both SQL adapters put
// a result through.
//
// It is also the one read that starts from the collection's *own* fields rather than from the
// documents alone, which is what gives a `find` matching nothing the same header as one matching
// everything. `SELECT * FROM t` on an empty table still names its columns; this is that.
func (s *session) find(
	ctx context.Context,
	query driver.Query,
	one statement,
	read cursorBounds,
) (driver.QueryResult, error) {
	known, err := s.fields(ctx, query.Namespace, one.Collection)
	if err != nil {
		return driver.QueryResult{}, err
	}

	opts := options.Find()
	if len(read.Sort) > 0 {
		opts.SetSort(read.Sort)
	}
	if read.Skip > 0 {
		opts.SetSkip(read.Skip)
	}
	if read.Limit > 0 {
		opts.SetLimit(read.Limit)
	}
	// A projection changes the shape, so the collection's own fields are no longer what the result
	// has: it then answers with whatever the documents carried, like every other read here.
	projection, projected := one.Arg(1).(bson.D)
	if projected {
		opts.SetProjection(projection)
		known = nil
	}

	result, err := s.read(ctx, query.MaxRows, known, func() (*mgo.Cursor, error) {
		return s.collection(query.Namespace, one.Collection).Find(ctx, one.Filter(), opts)
	})
	if err != nil {
		return driver.QueryResult{}, err
	}

	s.attachSource(ctx, query.Namespace, one.Collection, known, &result)
	return result, nil
}

// attachSource marks a result writable. The grid needs the collection to address a document and the
// key to identify it; a read that cannot carry `_id` back is left read-only rather than guessed at.
func (s *session) attachSource(
	ctx context.Context,
	ns driver.Namespace,
	collection string,
	known []driver.Column,
	result *driver.QueryResult,
) {
	if known == nil {
		var err error
		if known, err = s.fields(ctx, ns, collection); err != nil {
			return
		}
	}
	if !sqlkit.MergeSchema(result.Columns, known, []string{idField}) {
		return
	}

	found := ns
	if found.Database == "" {
		found.Database = s.databaseName(ns)
	}
	result.Namespace, result.Table = found, collection
}

// cursor reads whatever a command's cursor produced, its shape entirely the command's: a pipeline
// answers with the documents it built, and a collection's own fields would be the wrong header for
// them. The cap is the caller's `MaxRows`, which is what keeps a read with no bounds of its own from
// being the whole collection.
func (s *session) cursor(
	ctx context.Context,
	maxRows int,
	open func() (*mgo.Cursor, error),
) (driver.QueryResult, error) {
	return s.read(ctx, maxRows, nil, open)
}

// read is cursor with a header to start from — the collection's own fields, for the one command whose
// documents are that collection's; see find.
func (s *session) read(
	ctx context.Context,
	maxRows int,
	known []driver.Column,
	open func() (*mgo.Cursor, error),
) (driver.QueryResult, error) {
	found, err := open()
	if err != nil {
		return driver.QueryResult{}, translateError(err)
	}

	rows, err := collect(ctx, found, known, maxRows, nil)
	if err != nil {
		return driver.QueryResult{}, err
	}
	return driver.QueryResult{
		Columns:   rows.Columns,
		Rows:      rows.Rows,
		Truncated: rows.Truncated,
	}, nil
}

func (s *session) aggregated(
	ctx context.Context,
	maxRows int,
	collection *mgo.Collection,
	stage bson.D,
) (driver.QueryResult, error) {
	return s.cursor(ctx, maxRows, func() (*mgo.Cursor, error) {
		return collection.Aggregate(ctx, []bson.D{stage})
	})
}

// distinct is the one read whose answer is a list rather than documents, so it is turned into a
// one-column result — which is what the grid draws it as.
func (s *session) distinct(
	ctx context.Context,
	query driver.Query,
	one statement,
) (driver.QueryResult, error) {
	name, ok := one.Arg(0).(string)
	if !ok {
		return driver.QueryResult{}, statementError("distinct takes a field name and, optionally, a filter")
	}
	filter, _ := one.Arg(1).(bson.D)

	pipeline := []bson.D{
		{{Key: "$match", Value: filterOrAll(filter)}},
		{{Key: "$group", Value: bson.D{{Key: idField, Value: "$" + name}}}},
		{{Key: "$sort", Value: bson.D{{Key: idField, Value: 1}}}},
	}

	result, err := s.cursor(ctx, query.MaxRows, func() (*mgo.Cursor, error) {
		return s.collection(query.Namespace, one.Collection).Aggregate(ctx, pipeline)
	})
	if err != nil {
		return driver.QueryResult{}, err
	}

	// The group's key is the value, so the column is named after the field it came from rather than
	// after the `_id` the pipeline had to call it.
	for i := range result.Columns {
		if result.Columns[i].Name == idField {
			result.Columns[i].Name = name
			result.Columns[i].PrimaryKey = false
		}
	}
	return result, nil
}

func filterOrAll(filter bson.D) bson.D {
	if filter == nil {
		return bson.D{}
	}
	return filter
}

func (s *session) update(
	ctx context.Context,
	collection *mgo.Collection,
	one statement,
) (driver.QueryResult, error) {
	update := one.Arg(1)
	if update == nil {
		return driver.QueryResult{}, statementError("an update takes a filter and what to change")
	}

	// A pipeline update is a list of stages rather than a document, and the server takes either —
	// which is how `$convert` reaches a field; see AlterColumn.
	if list, ok := update.(bson.A); ok {
		pipeline, err := stages(list)
		if err != nil {
			return driver.QueryResult{}, err
		}
		update = pipeline
	}

	// The third argument is the shell's options document, and `upsert` is the one of them that
	// changes what the command *does* rather than how it runs — it is what makes an import's
	// "add it if it is not there" a single command instead of a read and then a write.
	upsert := false
	if given, ok := one.Arg(2).(bson.D); ok {
		for _, field := range given {
			if asked, ok := field.Value.(bool); ok && strings.EqualFold(field.Key, "upsert") {
				upsert = asked
			}
		}
	}

	many := strings.EqualFold(one.Method, "updateMany")
	outcome := &mgo.UpdateResult{}
	var err error
	if many {
		outcome, err = collection.UpdateMany(ctx, one.Filter(), update,
			options.UpdateMany().SetUpsert(upsert))
	} else {
		outcome, err = collection.UpdateOne(ctx, one.Filter(), update,
			options.UpdateOne().SetUpsert(upsert))
	}
	if err != nil {
		return driver.QueryResult{}, translateError(err)
	}
	return affected(outcome.ModifiedCount + outcome.UpsertedCount), nil
}

func (s *session) createIndex(
	ctx context.Context,
	collection *mgo.Collection,
	one statement,
) (driver.QueryResult, error) {
	keys, ok := one.Arg(0).(bson.D)
	if !ok {
		return driver.QueryResult{}, statementError("createIndex takes a document of keys")
	}

	model := mgo.IndexModel{Keys: keys}
	if given, ok := one.Arg(1).(bson.D); ok {
		opts := options.Index()
		for _, field := range given {
			switch strings.ToLower(field.Key) {
			case "name":
				if name, ok := field.Value.(string); ok {
					opts.SetName(name)
				}
			case "unique":
				if unique, ok := field.Value.(bool); ok {
					opts.SetUnique(unique)
				}
			case "sparse":
				if sparse, ok := field.Value.(bool); ok {
					opts.SetSparse(sparse)
				}
			}
		}
		model.Options = opts
	}

	if _, err := collection.Indexes().CreateOne(ctx, model); err != nil {
		return driver.QueryResult{}, translateError(err)
	}
	return affected(0), nil
}

func (s *session) createView(
	ctx context.Context,
	ns driver.Namespace,
	one statement,
) (driver.QueryResult, error) {
	name, _ := one.Arg(0).(string)
	on, _ := one.Arg(1).(string)
	if name == "" || on == "" {
		return driver.QueryResult{}, statementError(
			"createView takes the view's name, the collection it reads and a pipeline")
	}

	pipeline, err := stages(one.Arg(2))
	if err != nil {
		return driver.QueryResult{}, err
	}

	err = s.database(ns).CreateView(ctx, name, on, pipeline)
	return affected(0), translateError(err)
}

// returning is a command that answers with the document it acted on. A command that matched nothing
// answers with no rows rather than with a failure — the same thing a `find` that matched nothing does.
func (s *session) returning(_ context.Context, result *mgo.SingleResult) (driver.QueryResult, error) {
	raw, err := result.Raw()
	if err != nil {
		if err == mgo.ErrNoDocuments {
			return driver.QueryResult{Rows: [][]*string{}}, nil
		}
		return driver.QueryResult{}, translateError(err)
	}
	return oneDocument(raw), nil
}

// counted turns a count into the one-row, one-column result the grid draws it as. It takes the two
// values a count answers with so the call reads as one line: `counted(coll.CountDocuments(…))`.
func counted(total int64, err error) (driver.QueryResult, error) {
	if err != nil {
		return driver.QueryResult{}, translateError(err)
	}
	return driver.QueryResult{
		Columns: []driver.Column{{Name: "count", Type: "long", FullType: "long"}},
		Rows:    [][]*string{{cellText(strconv.FormatInt(total, 10))}},
	}, nil
}

// explainPlan asks the server what it would do. Only the two reads can be explained, because only
// they have a plan: everything else either writes or is a command whose plan is itself, and that is
// `UNSUPPORTED` rather than a plan invented here.
//
// The plan's own rows are the answer as one document — the same one row the grid drew when Explain
// was all this did — and the rest is that document read as stages; see readPlan.
func (s *session) explainPlan(
	ctx context.Context,
	ns driver.Namespace,
	one statement,
) (*driver.Plan, error) {
	read, err := bounds(one)
	if err != nil {
		return nil, err
	}

	inner := bson.D{}
	switch strings.ToLower(one.Method) {
	case "find", "findone":
		inner = bson.D{
			{Key: "find", Value: one.Collection},
			{Key: "filter", Value: filterOrAll(one.Filter())},
		}
		if len(read.Sort) > 0 {
			inner = append(inner, bson.E{Key: "sort", Value: read.Sort})
		}
		if read.Skip > 0 {
			inner = append(inner, bson.E{Key: "skip", Value: read.Skip})
		}
		if read.Limit > 0 {
			inner = append(inner, bson.E{Key: "limit", Value: read.Limit})
		}

	case "aggregate":
		pipeline, err := stages(one.Arg(0))
		if err != nil {
			return nil, err
		}
		inner = bson.D{
			{Key: "aggregate", Value: one.Collection},
			{Key: "pipeline", Value: pipeline},
			{Key: "cursor", Value: bson.D{}},
		}

	default:
		return nil, domain.Unsupported("explaining " + one.Method)
	}

	raw, err := s.runCommand(ctx, ns, bson.D{
		{Key: "explain", Value: inner},
		{Key: "verbosity", Value: "queryPlanner"},
	})
	if err != nil {
		return nil, err
	}

	plan := readPlan(raw)
	if plan == nil {
		plan = sqlkit.EmptyPlan()
	}

	document := oneDocument(raw)
	plan.Columns, plan.Rows = document.Columns, document.Rows
	return plan, nil
}

// stages reads a list of documents — a pipeline, or the documents of an `insertMany`. A single
// document is taken as a list of one, which is what the shell does for a pipeline of one stage.
func stages(value any) ([]bson.D, error) {
	switch value := value.(type) {
	case nil:
		return []bson.D{}, nil
	case bson.D:
		return []bson.D{value}, nil
	case bson.A:
		out := make([]bson.D, 0, len(value))
		for _, item := range value {
			document, ok := item.(bson.D)
			if !ok {
				return nil, statementError("a pipeline holds documents")
			}
			out = append(out, document)
		}
		return out, nil
	default:
		return nil, statementError("expected a document or a list of them")
	}
}

// oneDocument is a command's answer as the one-row result the grid draws it as: its fields are the
// columns, in the order the server wrote them.
func oneDocument(raw bson.Raw) driver.QueryResult {
	found := newFieldSet()
	found.addDocument(raw)
	columns := found.list()

	return driver.QueryResult{Columns: columns, Rows: [][]*string{row(raw, columns)}}
}

// affected is a command that changed things rather than returning any: the count the output panel
// reports, and no rows at all.
func affected(count int64) driver.QueryResult {
	return driver.QueryResult{Rows: [][]*string{}, RowsAffected: count}
}

func unknownMethod(method string) error {
	return statementError("`" + method + "` is not a command this side knows — " +
		"anything else can be run as db.runCommand({…})")
}
