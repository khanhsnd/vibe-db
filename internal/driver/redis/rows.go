package redis

import (
	"context"
	"strconv"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// Editing a key space is editing keys, and the columns a reader can change are different commands
// rather than clauses of one. That is the whole of this file: a value is `SET` or a rebuild, a clock
// is `EXPIRE` or `PERSIST`, a name is `RENAME`, and what a key *is* is not a property of the key at
// all but of what it holds — so a type is changed by writing the value as that type, and travels
// with it.

// UpdateCell changes one column of one key.
//
// `ValueType` is the one thing here that is not a column: a key's type and its value are **one fact**,
// so they are written together. Writing them apart could not work in either order — a value in the new
// type's shape is refused by the old type's parser, and a type change on its own has nothing but the
// old value to re-express — which is why the type is not a cell that can be written and the panel
// sends it alongside the value instead.
func (s *session) UpdateCell(ctx context.Context, edit driver.CellEdit) error {
	client, err := s.client(edit.Namespace)
	if err != nil {
		return err
	}
	if !isKeySpace(edit.Table) {
		return domain.ErrUnknownTable
	}

	key, err := keyOf(edit.Key)
	if err != nil {
		return err
	}
	column, known := canonical(edit.Column)
	if !known {
		return unknownColumn(edit.Column)
	}

	switch column {
	case columnKey:
		return rename(ctx, client, key, edit.Value)
	case columnTTL:
		return expire(ctx, client, key, edit.Value)
	case columnValue:
		if edit.Value == nil {
			return valueError("cannot be nothing: a key that holds nothing does not exist, so " +
				"clearing one is deleting it")
		}
		kind, err := writtenAs(ctx, client, key, edit.ValueType)
		if err != nil {
			return err
		}
		return writeValue(ctx, client, key, kind, *edit.Value)
	case columnType:
		return domain.Unsupported("changing a type on its own: a key's type is what its value is, " +
			"so the two are written together and the value is what carries it")
	}

	return domain.Unsupported("changing what a key costs")
}

// writtenAs is the type a value is about to be written as: the one the reader picked, or the one the
// key already is where they picked none. `writeValue` empties the key before it fills it, so a type
// the reader *did* pick simply replaces what was there — which is the whole of what changing a key's
// type means on this engine. A type this adapter cannot write is refused here rather than after the
// key has been emptied, and a value the new type cannot read is refused by `parseValue` before a
// single command is sent.
func writtenAs(
	ctx context.Context,
	client *goredis.Client,
	key string,
	picked string,
) (string, error) {
	if wanted := strings.ToLower(strings.TrimSpace(picked)); wanted != "" {
		if !writable[wanted] {
			return "", domain.Unsupported("writing a " + wanted + " value")
		}
		return wanted, nil
	}

	kind, err := client.Type(ctx, key).Result()
	if err != nil {
		return "", translateError(err)
	}
	if kind == typeMissing {
		return "", domain.ErrUnknownTable
	}
	return kind, nil
}

// writable is the five types a value can be *written* as. A stream is the sixth type there is and is
// not among them: its ids are the server's and it is append-only, which is the same refusal
// `parseValue` already makes — said here so that picking it from a list fails before the key is
// emptied rather than after.
var writable = map[string]bool{
	typeString: true,
	typeList:   true,
	typeSet:    true,
	typeHash:   true,
	typeZSet:   true,
}

// keyOf reads the one column that identifies a row. `sqlkit.EditableKeys` is the SQL side of this
// rule and refuses the same two things: a row with no key, and a key holding nothing.
func keyOf(held map[string]string) (string, error) {
	for name, value := range held {
		if column, known := canonical(name); known && column == columnKey && value != "" {
			return value, nil
		}
	}
	return "", domain.ErrTableNotEditable
}

// rename moves a key. It is `RENAMENX` and not `RENAME` on purpose: `RENAME` takes the name whatever
// is already there, which would make a mistyped edit a silent deletion of somebody else's key.
func rename(ctx context.Context, client *goredis.Client, key string, name *string) error {
	if name == nil || *name == "" {
		return valueError("cannot be nothing: a key is what names a row here")
	}
	if *name == key {
		return nil
	}

	moved, err := client.RenameNX(ctx, key, *name).Result()
	if err != nil {
		return translateError(err)
	}
	if !moved {
		return domain.StatementError{
			Code:    "BUSYKEY",
			Message: "a key called " + *name + " is already there",
		}
	}
	return nil
}

// expire sets or clears the clock on a key. Clearing it is `PERSIST` and not an expiry of zero,
// which the server reads as *now* and answers by deleting the key — the one place a plausible
// reading of an empty box would destroy data.
func expire(ctx context.Context, client *goredis.Client, key string, ttl *string) error {
	if ttl == nil || strings.TrimSpace(*ttl) == "" {
		return translateError(client.Persist(ctx, key).Err())
	}

	value, err := strconv.ParseInt(strings.TrimSpace(*ttl), 10, 64)
	if err != nil || value <= 0 {
		return valueError("is a number of seconds above zero; clear it to take the expiry off")
	}
	return translateError(client.Expire(ctx, key, time.Duration(value)*time.Second).Err())
}

// InsertRow writes a key that is not there yet. It refuses one that is, because that is what
// inserting a row means: a `SET` over a key the reader could not see would be an edit they did not
// make, and the row is already in the grid to be edited if they want it.
func (s *session) InsertRow(ctx context.Context, insert driver.RowInsert) error {
	client, err := s.client(insert.Namespace)
	if err != nil {
		return err
	}
	if !isKeySpace(insert.Table) {
		return domain.ErrUnknownTable
	}

	values := map[string]*string{}
	for name, value := range insert.Values {
		column, known := canonical(name)
		if !known {
			return unknownColumn(name)
		}
		values[column] = value
	}

	key := values[columnKey]
	if key == nil || *key == "" {
		return valueError("of a new row is its key, and it needs one")
	}

	exists, err := client.Exists(ctx, *key).Result()
	if err != nil {
		return translateError(err)
	}
	if exists > 0 {
		return domain.StatementError{
			Code:    "BUSYKEY",
			Message: "a key called " + *key + " is already there",
		}
	}

	// A key with no type named is a string, which is what every other client assumes and what a
	// reader typing a value into a new row means.
	kind := typeString
	if named := values[columnType]; named != nil && strings.TrimSpace(*named) != "" {
		kind = strings.ToLower(strings.TrimSpace(*named))
	}

	value := ""
	if held := values[columnValue]; held != nil {
		value = *held
	}
	if err := writeValue(ctx, client, *key, kind, value); err != nil {
		return err
	}

	if ttl := values[columnTTL]; ttl != nil && strings.TrimSpace(textOf(ttl)) != "" {
		return expire(ctx, client, *key, ttl)
	}
	return nil
}

// DeleteRow removes a key. It is `UNLINK` rather than `DEL`: the two say the same thing to the
// reader and only one of them hands the freeing of a million-element list to a background thread
// instead of stopping the server with it.
func (s *session) DeleteRow(ctx context.Context, removal driver.RowDelete) error {
	client, err := s.client(removal.Namespace)
	if err != nil {
		return err
	}
	if !isKeySpace(removal.Table) {
		return domain.ErrUnknownTable
	}

	key, err := keyOf(removal.Key)
	if err != nil {
		return err
	}
	return translateError(client.Unlink(ctx, key).Err())
}
