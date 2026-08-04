package mongo

import (
	"context"
	"strconv"

	"go.mongodb.org/mongo-driver/v2/bson"
	mgo "go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/khanhsnd/vibe-db/internal/driver"
)

// idIndex is what MongoDB calls the index behind `_id`; it has no other name, and it is the index
// the server makes whether or not anybody asked for one.
const idIndex = "_id_"

// TableStructure is a collection's shape rather than its documents: the fields the sample found, and
// the indexes the server really keeps.
//
// **There are never any foreign keys.** MongoDB has no referential constraints of any kind, so this
// answers with none rather than with something invented — and `CreateForeignKey` says so out loud
// rather than letting a server reject it.
func (s *session) TableStructure(
	ctx context.Context,
	ns driver.Namespace,
	table string,
) (driver.TableStructure, error) {
	fields, err := s.fields(ctx, ns, table)
	if err != nil {
		return driver.TableStructure{}, err
	}

	indexes, err := s.indexes(ctx, ns, table)
	if err != nil {
		return driver.TableStructure{}, err
	}

	return driver.TableStructure{
		Columns:     fields,
		Indexes:     indexes,
		ForeignKeys: []driver.ForeignKey{},
	}, nil
}

// indexes reads what the server keeps. An index's *keys document* carries both the fields and how
// each is indexed — `1`, `-1`, `"hashed"`, `"text"`, `"2dsphere"` — so the method is read out of the
// value rather than out of a column of its own, which is where MongoDB keeps what SQL calls USING.
func (s *session) indexes(
	ctx context.Context,
	ns driver.Namespace,
	table string,
) ([]driver.Index, error) {
	found, err := s.collection(ns, table).Indexes().ListSpecifications(ctx)
	if err != nil {
		return nil, translateError(err)
	}

	indexes := make([]driver.Index, 0, len(found))
	for _, one := range found {
		columns, method := indexKeys(one.KeysDocument)

		indexes = append(indexes, driver.Index{
			Name:    one.Name,
			Columns: columns,
			Unique:  one.Unique != nil && *one.Unique,
			Primary: one.Name == idIndex,
			Method:  method,
		})
	}

	// The index behind `_id` comes first, which is where MySQL's PRIMARY is ordered to as well.
	for i, one := range indexes {
		if one.Primary && i > 0 {
			indexes[0], indexes[i] = indexes[i], indexes[0]
			break
		}
	}

	return indexes, nil
}

// indexKeys reads a keys document into the fields it names and the one method they share. An
// ordinary ascending or descending index has no method to report — that is what an empty one means
// everywhere else in the app — and a special one names itself in the key's value.
func indexKeys(keys bson.Raw) ([]string, string) {
	elements, err := keys.Elements()
	if err != nil {
		return []string{}, ""
	}

	columns := make([]string, 0, len(elements))
	method := ""
	for _, element := range elements {
		name, err := element.KeyErr()
		if err != nil {
			continue
		}
		columns = append(columns, name)

		if special, ok := element.Value().StringValueOK(); ok {
			method = special
		}
	}
	return columns, method
}

// TableSchemas is what the editor completes from: every collection of a database and the fields its
// documents were found to carry.
//
// It is the one read in this adapter that costs a round trip per collection, because a document store
// has no `information_schema.columns` to answer for all of them at once. Each of those trips is a
// small one — see completionDocs — and it happens once when a database is opened rather than as
// anybody types.
func (s *session) TableSchemas(
	ctx context.Context,
	ns driver.Namespace,
) ([]driver.TableSchema, error) {
	found, err := s.collections(ctx, ns)
	if err != nil {
		return nil, err
	}

	tables := make([]driver.TableSchema, 0, len(found))
	for _, one := range found {
		names, err := s.fieldNames(ctx, ns, one.Name)
		if err != nil {
			return nil, err
		}
		tables = append(tables, driver.TableSchema{Name: one.Name, Columns: names})
	}
	return tables, nil
}

func (s *session) fieldNames(
	ctx context.Context,
	ns driver.Namespace,
	collection string,
) ([]string, error) {
	found, err := s.sample(ctx, ns, collection, completionDocs)
	if err != nil {
		return nil, err
	}

	names := []string{}
	for _, one := range found.list() {
		names = append(names, one.Name)
	}
	return names, nil
}

// TableList is the collections a database holds with what the server keeps about each of them.
//
// It is **one read per collection**, which is the one place this adapter is more expensive than a
// SQL one: `$collStats` is a per-collection aggregation and there is no catalogue table to join
// against. A view is skipped — it has no storage of its own to report — and its row carries only its
// name and its kind.
func (s *session) TableList(ctx context.Context, ns driver.Namespace) (driver.TableList, error) {
	found, err := s.collections(ctx, ns)
	if err != nil {
		return driver.TableList{}, err
	}

	rows := make([][]*string, 0, len(found))
	for _, one := range found {
		stats := collectionStats{}
		if one.Type != "view" {
			stats, err = s.stats(ctx, ns, one.Name)
			if err != nil {
				return driver.TableList{}, err
			}
		}
		rows = append(rows, stats.row(one))
	}

	return driver.TableList{Columns: tableListColumns, Rows: rows}, nil
}

// tableListColumns is what the grid draws as its headers. They are this engine's own facts, for the
// reason `driver.TableList` gives: no two engines keep the same things about a table, and a column
// neither could fill would say nothing.
var tableListColumns = []driver.Column{
	{Name: "Name", Type: "string", FullType: "string"},
	{Name: "Kind", Type: "string", FullType: "string"},
	{Name: "Documents", Type: "long", FullType: "long", Nullable: true},
	{Name: "Data Size", Type: "string", FullType: "string", Nullable: true},
	{Name: "Storage Size", Type: "string", FullType: "string", Nullable: true},
	{Name: "Avg Document Size", Type: "string", FullType: "string", Nullable: true},
	{Name: "Indexes", Type: "int", FullType: "int", Nullable: true},
	{Name: "Index Size", Type: "string", FullType: "string", Nullable: true},
}

// collectionStats is what `$collStats` says about one collection's storage.
type collectionStats struct {
	Count       int64 `bson:"count"`
	Size        int64 `bson:"size"`
	StorageSize int64 `bson:"storageSize"`
	AvgObjSize  int64 `bson:"avgObjSize"`
	IndexCount  int32 `bson:"nindexes"`
	IndexSize   int64 `bson:"totalIndexSize"`
	// read says the stats were actually gathered. A view has none, and its row then carries nulls
	// rather than a row of zeroes claiming an empty collection.
	read bool
}

func (c collectionStats) row(spec mgo.CollectionSpecification) []*string {
	kind := spec.Type
	if kind == "" {
		kind = "collection"
	}

	row := []*string{cellText(spec.Name), cellText(kind)}
	if !c.read {
		return append(row, nil, nil, nil, nil, nil, nil)
	}

	return append(row,
		cellText(strconv.FormatInt(c.Count, 10)),
		cellText(sizeText(c.Size)),
		cellText(sizeText(c.StorageSize)),
		cellText(sizeText(c.AvgObjSize)),
		cellText(strconv.FormatInt(int64(c.IndexCount), 10)),
		cellText(sizeText(c.IndexSize)),
	)
}

// cellText is one cell of a row this side spelled rather than read off a document, which is the one
// shape in the app where a value is known never to be absent.
func cellText(text string) *string {
	return &text
}

// stats reads one collection's storage. `$collStats` is the aggregation stage rather than the
// `collStats` command, which the server has deprecated.
func (s *session) stats(
	ctx context.Context,
	ns driver.Namespace,
	collection string,
) (collectionStats, error) {
	pipeline := []bson.D{
		{{Key: "$collStats", Value: bson.D{{Key: "storageStats", Value: bson.D{}}}}},
	}

	cursor, err := s.collection(ns, collection).Aggregate(ctx, pipeline)
	if err != nil {
		return collectionStats{}, translateError(err)
	}
	defer cursor.Close(ctx)

	if !cursor.Next(ctx) {
		return collectionStats{}, translateError(cursor.Err())
	}

	var answer struct {
		StorageStats collectionStats `bson:"storageStats"`
	}
	if err := cursor.Decode(&answer); err != nil {
		return collectionStats{}, translateError(err)
	}

	stats := answer.StorageStats
	stats.read = true
	return stats, nil
}
