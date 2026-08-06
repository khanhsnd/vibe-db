// Package compare works out the difference between two tables: their shapes, or their rows.
//
// It knows no engines and opens no connections. Both sides arrive already read — the columns,
// indexes and constraints a driver reported, and the rows it handed over as `[][]*string` — and
// what comes back is those same facts *paired*, one row of the answer per thing compared. That is
// the whole reason it is a package of its own rather than something `internal/app` does on the way
// past: a diff is a rule about data and not a wire format, and it is worth being able to test
// without a server. It sits exactly where `internal/transfer` sits, and for the same reason.
//
// **The left is what the reader has and the right is what they are comparing it against.** Added
// means the right holds it and the left does not; Removed means the left holds it and the right
// does not. Nothing here decides which table is which — the caller does, and the wording follows.
//
// **Nothing is matched by position.** A field is matched by its name and a row by its key, because
// a column inserted in the middle would otherwise report every column below it as changed, and two
// tables read in two different orders would agree about nothing at all.
package compare

import (
	"strconv"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/driver"
)

// Status is what the comparison says about one field, one index, one constraint or one row. The
// list is closed: a thing is on both sides and alike, on both sides and not, or on one side only.
type Status string

const (
	Same    Status = "same"
	Changed Status = "changed"
	// Added is on the right and not on the left; Removed is the other way about.
	Added   Status = "added"
	Removed Status = "removed"
)

// The names of the facts a difference is reported in. They are the words the reader is shown, so
// they are spelled once here rather than in whatever draws them.
const (
	diffType       = "type"
	diffNullable   = "nullable"
	diffDefault    = "default"
	diffPrimaryKey = "primary key"
	diffFields     = "fields"
	diffUnique     = "unique"
	diffMethod     = "method"
	diffSchema     = "referenced schema"
	diffTable      = "referenced table"
	diffReferences = "referenced fields"
	diffOnUpdate   = "on update"
	diffOnDelete   = "on delete"
)

// FieldPair is one field of the comparison: whichever sides have it, and what differs between them.
// Left and Right are nil for a field the other side alone holds, which is what a blank opposite an
// Added or a Removed one is drawn from.
type FieldPair struct {
	Name    string
	Left    *driver.Column
	Right   *driver.Column
	Status  Status
	Differs []string
}

// IndexPair is one index of the comparison, matched by name for the reason a field is: an index
// renamed is a different index, and an index whose columns moved is the same one changed.
type IndexPair struct {
	Name    string
	Left    *driver.Index
	Right   *driver.Index
	Status  Status
	Differs []string
}

// ForeignKeyPair is one referential constraint of the comparison.
type ForeignKeyPair struct {
	Name    string
	Left    *driver.ForeignKey
	Right   *driver.ForeignKey
	Status  Status
	Differs []string
}

// Structure is two tables' shapes lined up: one entry per field, index and constraint either side
// holds, in the left's own reading order with what only the right holds set where the right has it.
type Structure struct {
	Fields      []FieldPair
	Indexes     []IndexPair
	ForeignKeys []ForeignKeyPair
}

// Table is one side of a data comparison as it was read: the columns the driver reported, the rows
// under them, and whether there were more rows than the read asked for.
type Table struct {
	Columns   []driver.Column
	Rows      [][]*string
	Truncated bool
}

// RowPair is one row of the comparison. Left and Right hold a value per compared column, in the
// order Data.Columns names them, and one of the two is nil for a row only the other side has.
type RowPair struct {
	Status Status
	// Key is the values the two sides were matched on, in the order Data.Key names them.
	Key   []string
	Left  []*string
	Right []*string
	// Differs is one flag per compared column, true where the sides hold different values. It is
	// empty for a row only one side holds, there being nothing to have compared it against.
	Differs []bool
}

// Key is the columns two tables' rows are matched on. Declared says they are the tables' own
// primary key rather than the fallback, and travels with them because the two facts must never
// drift apart: on the fallback nothing can be Changed — a row that differs anywhere is a row
// neither side has — and a reader shown a diff of two unkeyed tables has to be told that.
type Key struct {
	Columns  []string
	Declared bool
}

// Data is two tables' rows lined up.
type Data struct {
	Key Key
	// Columns is the columns both sides hold, in the left's order, and the only ones compared.
	Columns []string
	// The columns one side holds alone. They are named rather than compared: a value with nothing
	// opposite it is not a difference between two rows, it is a difference between two shapes, and
	// the structure comparison is where that is read.
	LeftOnly  []string
	RightOnly []string
	Rows      []RowPair
	// What each side's read actually returned, and whether it was cut short — a bounded compare
	// answers about the rows it read and must say so rather than implying it saw the table.
	LeftRows       int
	RightRows      int
	LeftTruncated  bool
	RightTruncated bool
}

// CompareStructure lines two tables' shapes up.
func CompareStructure(left, right driver.TableStructure) Structure {
	return Structure{
		Fields:      compareFields(left.Columns, right.Columns),
		Indexes:     compareIndexes(left.Indexes, right.Indexes),
		ForeignKeys: compareForeignKeys(left.ForeignKeys, right.ForeignKeys),
	}
}

func compareFields(left, right []driver.Column) []FieldPair {
	pairs := make([]FieldPair, 0, len(left)+len(right))

	for _, at := range pairNames(names(left, columnName), names(right, columnName)) {
		one := FieldPair{}
		if at.Left >= 0 {
			one.Left = &left[at.Left]
			one.Name = left[at.Left].Name
		}
		if at.Right >= 0 {
			one.Right = &right[at.Right]
			one.Name = right[at.Right].Name
		}

		one.Differs = fieldDiffers(one.Left, one.Right)
		one.Status = statusOf(one.Left != nil, one.Right != nil, len(one.Differs) > 0)
		pairs = append(pairs, one)
	}

	return pairs
}

// fieldDiffers names what differs between two columns of one name. The type is read from
// `FullType` where the engine gave one — `varchar(255)` says more than `varchar` — and every
// comparison here folds case, because two engines spell one type two ways and neither is wrong.
func fieldDiffers(left, right *driver.Column) []string {
	if left == nil || right == nil {
		return nil
	}

	var differs []string
	if !alike(fullType(*left), fullType(*right)) {
		differs = append(differs, diffType)
	}
	if left.Nullable != right.Nullable {
		differs = append(differs, diffNullable)
	}
	if !alikeDefault(left.Default, right.Default) {
		differs = append(differs, diffDefault)
	}
	if left.PrimaryKey != right.PrimaryKey {
		differs = append(differs, diffPrimaryKey)
	}
	return differs
}

func compareIndexes(left, right []driver.Index) []IndexPair {
	pairs := make([]IndexPair, 0, len(left)+len(right))

	for _, at := range pairNames(names(left, indexName), names(right, indexName)) {
		one := IndexPair{}
		if at.Left >= 0 {
			one.Left = &left[at.Left]
			one.Name = left[at.Left].Name
		}
		if at.Right >= 0 {
			one.Right = &right[at.Right]
			one.Name = right[at.Right].Name
		}

		one.Differs = indexDiffers(one.Left, one.Right)
		one.Status = statusOf(one.Left != nil, one.Right != nil, len(one.Differs) > 0)
		pairs = append(pairs, one)
	}

	return pairs
}

// indexDiffers reads an index's columns in order, since an index over `(a, b)` is not the one over
// `(b, a)`. Whether it is the primary key's own index is not compared: that is the field's
// `primary key` fact said a second time, and reporting it twice reads as two differences.
func indexDiffers(left, right *driver.Index) []string {
	if left == nil || right == nil {
		return nil
	}

	var differs []string
	if !alikeLists(left.Columns, right.Columns) {
		differs = append(differs, diffFields)
	}
	if left.Unique != right.Unique {
		differs = append(differs, diffUnique)
	}
	if !alike(left.Method, right.Method) {
		differs = append(differs, diffMethod)
	}
	return differs
}

func compareForeignKeys(left, right []driver.ForeignKey) []ForeignKeyPair {
	pairs := make([]ForeignKeyPair, 0, len(left)+len(right))

	for _, at := range pairNames(names(left, foreignKeyName), names(right, foreignKeyName)) {
		one := ForeignKeyPair{}
		if at.Left >= 0 {
			one.Left = &left[at.Left]
			one.Name = left[at.Left].Name
		}
		if at.Right >= 0 {
			one.Right = &right[at.Right]
			one.Name = right[at.Right].Name
		}

		one.Differs = foreignKeyDiffers(one.Left, one.Right)
		one.Status = statusOf(one.Left != nil, one.Right != nil, len(one.Differs) > 0)
		pairs = append(pairs, one)
	}

	return pairs
}

func foreignKeyDiffers(left, right *driver.ForeignKey) []string {
	if left == nil || right == nil {
		return nil
	}

	var differs []string
	if !alikeLists(left.Columns, right.Columns) {
		differs = append(differs, diffFields)
	}
	if !alike(left.ReferencedSchema, right.ReferencedSchema) {
		differs = append(differs, diffSchema)
	}
	if !alike(left.ReferencedTable, right.ReferencedTable) {
		differs = append(differs, diffTable)
	}
	if !alikeLists(left.ReferencedColumns, right.ReferencedColumns) {
		differs = append(differs, diffReferences)
	}
	if !alike(left.OnUpdate, right.OnUpdate) {
		differs = append(differs, diffOnUpdate)
	}
	if !alike(left.OnDelete, right.OnDelete) {
		differs = append(differs, diffOnDelete)
	}
	return differs
}

// KeyFor is the columns two tables' rows are matched on: the columns of the primary key, where
// both sides declare the same one, and every shared column where they do not. The second answer is
// a fallback and says so, because a row matched on all of itself can only ever be present or
// absent, never changed.
//
// It is read from the *shapes* rather than from the rows, so a caller can order both reads by the
// key before it asks for a single row — which is what makes a bounded compare compare comparable
// windows of two tables instead of two arbitrary hundreds.
func KeyFor(left, right driver.TableStructure) Key {
	shared := sharedColumns(left.Columns, right.Columns)

	declared := make([]string, 0, len(shared))
	for _, name := range shared {
		if columnBy(left.Columns, name).PrimaryKey && columnBy(right.Columns, name).PrimaryKey {
			declared = append(declared, name)
		}
	}
	if len(declared) > 0 {
		return Key{Columns: declared, Declared: true}
	}
	return Key{Columns: shared}
}

// CompareRows lines two tables' rows up on `key`, which is what KeyFor answered for the same two
// tables. Only the columns both sides hold are compared, in the left's order.
//
// Rows are matched **in the order they were read**, and a key held more than once is matched
// first-to-first: a table with no key of its own is compared on the whole of a row, and two
// identical rows on one side then pair with two on the other rather than with one twice.
func CompareRows(left, right Table, key Key) Data {
	shared := sharedColumns(left.Columns, right.Columns)
	if len(key.Columns) == 0 {
		key = Key{Columns: shared}
	}

	data := Data{
		Key:            key,
		Columns:        shared,
		LeftOnly:       missingColumns(left.Columns, right.Columns),
		RightOnly:      missingColumns(right.Columns, left.Columns),
		Rows:           make([]RowPair, 0, len(left.Rows)),
		LeftRows:       len(left.Rows),
		RightRows:      len(right.Rows),
		LeftTruncated:  left.Truncated,
		RightTruncated: right.Truncated,
	}

	leftAt := positions(left.Columns, shared)
	rightAt := positions(right.Columns, shared)
	leftKeyAt := positions(left.Columns, key.Columns)
	rightKeyAt := positions(right.Columns, key.Columns)

	// Every right row that has not been paired yet, by key and in the order it was read. A queue
	// per key rather than one row per key, so duplicates pair off one for one.
	waiting := make(map[string][]int, len(right.Rows))
	for at, row := range right.Rows {
		encoded := encodeKey(pick(row, rightKeyAt))
		waiting[encoded] = append(waiting[encoded], at)
	}
	paired := make([]bool, len(right.Rows))

	for _, row := range left.Rows {
		values := pick(row, leftAt)
		keyValues := pick(row, leftKeyAt)
		encoded := encodeKey(keyValues)

		queue := waiting[encoded]
		if len(queue) == 0 {
			data.Rows = append(data.Rows, RowPair{
				Status: Removed,
				Key:    keyText(keyValues),
				Left:   values,
			})
			continue
		}

		at := queue[0]
		waiting[encoded] = queue[1:]
		paired[at] = true

		other := pick(right.Rows[at], rightAt)
		differs, changed := cellsDiffer(values, other)

		data.Rows = append(data.Rows, RowPair{
			Status:  statusOf(true, true, changed),
			Key:     keyText(keyValues),
			Left:    values,
			Right:   other,
			Differs: differs,
		})
	}

	for at, row := range right.Rows {
		if paired[at] {
			continue
		}
		data.Rows = append(data.Rows, RowPair{
			Status: Added,
			Key:    keyText(pick(row, rightKeyAt)),
			Right:  pick(row, rightAt),
		})
	}

	return data
}

// cellsDiffer says which of two rows' cells hold different values, and whether any of them do. A
// NULL is not the empty string here, exactly as it is not one anywhere else in the app.
func cellsDiffer(left, right []*string) ([]bool, bool) {
	differs := make([]bool, len(left))
	any := false

	for at := range left {
		var other *string
		if at < len(right) {
			other = right[at]
		}
		if !alikeDefault(left[at], other) {
			differs[at] = true
			any = true
		}
	}

	return differs, any
}

// statusOf is the one rule that turns "which sides hold it, and does it differ" into a status, so
// a field, an index, a constraint and a row all answer it the same way.
func statusOf(onLeft, onRight, differs bool) Status {
	switch {
	case !onRight:
		return Removed
	case !onLeft:
		return Added
	case differs:
		return Changed
	default:
		return Same
	}
}

// NamePair is where one thing sits on each side, and -1 for a side that has not got it.
type NamePair struct {
	Left  int
	Right int
}

// pairNames lines two ordered lists of names up so that a name both sides hold sits on one row and
// a name one side holds alone sits opposite a blank.
//
// It walks the left in its own order, because the left is the side the reader is reading down, and
// emits whatever the right holds *before* the next shared name and the left has not got at all —
// so a field inserted in the middle is drawn in the middle rather than pushed to the end. Anything
// still unpaired at the end is appended, which is what a name held twice on one side falls into.
func pairNames(left, right []string) []NamePair {
	rightAt := make(map[string]int, len(right))
	for at := len(right) - 1; at >= 0; at-- {
		rightAt[right[at]] = at
	}

	leftHas := make(map[string]bool, len(left))
	for _, name := range left {
		leftHas[name] = true
	}

	pairs := make([]NamePair, 0, len(left)+len(right))
	taken := make([]bool, len(right))
	next := 0

	// flush emits every right name up to `until` that the left has not got: the new ones, in the
	// place the right has them.
	flush := func(until int) {
		for ; next < until; next++ {
			if !taken[next] && !leftHas[right[next]] {
				pairs = append(pairs, NamePair{Left: -1, Right: next})
				taken[next] = true
			}
		}
	}

	for at, name := range left {
		other, both := rightAt[name]
		if !both || taken[other] {
			pairs = append(pairs, NamePair{Left: at, Right: -1})
			continue
		}

		flush(other)
		pairs = append(pairs, NamePair{Left: at, Right: other})
		taken[other] = true
	}

	for at := range right {
		if !taken[at] {
			pairs = append(pairs, NamePair{Left: -1, Right: at})
		}
	}

	return pairs
}

func columnName(column driver.Column) string      { return column.Name }
func indexName(index driver.Index) string         { return index.Name }
func foreignKeyName(key driver.ForeignKey) string { return key.Name }

func names[T any](list []T, of func(T) string) []string {
	out := make([]string, 0, len(list))
	for _, one := range list {
		out = append(out, of(one))
	}
	return out
}

// sharedColumns is the columns both sides hold, in the left's order — the left being the side the
// reader is reading down, here as everywhere else.
func sharedColumns(left, right []driver.Column) []string {
	held := make(map[string]bool, len(right))
	for _, column := range right {
		held[column.Name] = true
	}

	shared := make([]string, 0, len(left))
	for _, column := range left {
		if held[column.Name] {
			shared = append(shared, column.Name)
		}
	}
	return shared
}

// missingColumns is what `left` holds and `other` has not got, in the left's own order.
func missingColumns(left, other []driver.Column) []string {
	held := make(map[string]bool, len(other))
	for _, column := range other {
		held[column.Name] = true
	}

	only := make([]string, 0)
	for _, column := range left {
		if !held[column.Name] {
			only = append(only, column.Name)
		}
	}
	return only
}

func columnBy(columns []driver.Column, name string) driver.Column {
	for _, column := range columns {
		if column.Name == name {
			return column
		}
	}
	return driver.Column{}
}

// positions is where each of `wanted` sits in `columns`, and -1 for one it has not got — which
// `pick` then reads as a NULL, so a column missing from one side never shifts that side's values.
func positions(columns []driver.Column, wanted []string) []int {
	at := make(map[string]int, len(columns))
	for index, column := range columns {
		if _, seen := at[column.Name]; !seen {
			at[column.Name] = index
		}
	}

	out := make([]int, 0, len(wanted))
	for _, name := range wanted {
		if index, held := at[name]; held {
			out = append(out, index)
			continue
		}
		out = append(out, -1)
	}
	return out
}

func pick(row []*string, at []int) []*string {
	out := make([]*string, 0, len(at))
	for _, index := range at {
		if index < 0 || index >= len(row) {
			out = append(out, nil)
			continue
		}
		out = append(out, row[index])
	}
	return out
}

// keyText is a key as the reader reads it, a NULL spelled rather than dropped: a key of two
// columns where one is NULL is not the same key as one where the other is.
func keyText(values []*string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == nil {
			out = append(out, "")
			continue
		}
		out = append(out, *value)
	}
	return out
}

// encodeKey turns a row's key values into one comparable string. Each value carries its own length
// ahead of it, so no value can be read as the end of the one before it and `("a", "bc")` is never
// `("ab", "c")`; a NULL is a mark of its own, being neither a value nor an empty one.
func encodeKey(values []*string) string {
	var built strings.Builder
	for _, value := range values {
		if value == nil {
			built.WriteString("~|")
			continue
		}
		built.WriteString(strconv.Itoa(len(*value)))
		built.WriteByte(':')
		built.WriteString(*value)
		built.WriteByte('|')
	}
	return built.String()
}

// fullType is the type as the engine spelled it, with its length and modifiers where it gave them.
func fullType(column driver.Column) string {
	if column.FullType != "" {
		return column.FullType
	}
	return column.Type
}

// alike folds case and trims, because two engines spell one type, one access method and one
// referential action two ways and neither of them is wrong.
func alike(left, right string) bool {
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
}

func alikeLists(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for at := range left {
		if !alike(left[at], right[at]) {
			return false
		}
	}
	return true
}

// alikeDefault compares two values either of which may be absent. A column with no default and one
// defaulting to the empty string are not the same column, and a NULL cell is not an empty one —
// so the *presence* is compared before the text is. The text itself is compared verbatim: a
// default is an expression and `0` is not `'0'`.
func alikeDefault(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
