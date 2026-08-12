package sqlkit

import (
	"context"
	"database/sql"
	"sync"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// Txn is the whole of driver.Transaction for a `database/sql` adapter, and it is here for the
// reason DDL and Data are: nothing about holding one open varies by engine once the pinning is
// the adapter's. An adapter embeds it, hands it the same `pinned` that DDL is given, and gets
// Begin, Commit, Rollback and InTransaction without writing any of them.
//
// A transaction is **one connection**, taken out of the pool and kept until it ends. That is what
// makes it real: rows written inside it exist on that connection alone, so every statement of the
// session has to run there while it lasts or the reader would be shown a table their own writes
// are missing from. The two seams that make that true are Queryer, which DDL and Data ask before
// they reach for a pool, and Pinned, which is what the query path and a definition run on.
//
// Every method is safe on a nil receiver, so a Data or a DDL built without one — which is every
// test of the spelling — simply never has a transaction.
type Txn struct {
	// Pin takes one pooled connection and puts it inside a namespace, exactly as DDL.Pin does; it
	// is the adapter's own `pinned` in all five. An engine with nothing to pin still supplies it,
	// because a transaction needs the connection whether or not there is a namespace to be in.
	Pin func(ctx context.Context, ns driver.Namespace) (*sql.Conn, error)
	// Translate is the adapter's error map, since what a server says about a commit is said in
	// this engine's own words.
	Translate func(error) error

	mu   sync.Mutex
	conn *sql.Conn
	tx   *sql.Tx
}

// Begin takes the connection the transaction will live on and starts one on it. The namespace is
// the one the reader is looking at, so an unqualified name inside the transaction resolves where
// they expect — the same reason a definition is run inside its namespace rather than named into one.
func (t *Txn) Begin(ctx context.Context, ns driver.Namespace) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.tx != nil {
		return domain.ErrTransactionActive
	}

	conn, err := t.Pin(ctx, ns)
	if err != nil {
		return err
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		conn.Close()
		return t.fail(err)
	}

	t.conn, t.tx = conn, tx
	return nil
}

func (t *Txn) Commit(context.Context) error {
	return t.end((*sql.Tx).Commit)
}

func (t *Txn) Rollback(context.Context) error {
	return t.end((*sql.Tx).Rollback)
}

func (t *Txn) InTransaction() bool {
	return t.Queryer() != nil
}

// Queryer is the open transaction, and nil where there is none. It is what DDL and Data ask before
// they reach for a pool, and the nil is the whole switch: a session with nothing open runs exactly
// as it did before this existed.
func (t *Txn) Queryer() Queryer {
	if t == nil {
		return nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.tx == nil {
		return nil
	}
	return t.tx
}

// Pinned hands back what a statement runs on and the way to give it back: the open transaction,
// which is released by nothing, or a connection out of the pool put inside the namespace, which is
// released by being closed. It is what the query path and a definition go through, so neither has
// to know which of the two it got.
func (t *Txn) Pinned(ctx context.Context, ns driver.Namespace) (Queryer, func(), error) {
	if open := t.Queryer(); open != nil {
		return open, func() {}, nil
	}

	conn, err := t.Pin(ctx, ns)
	if err != nil {
		return nil, nil, err
	}
	return conn, func() { conn.Close() }, nil
}

// Close rolls back whatever is still open and gives the connection back. It is what a session's
// own Close calls first: a transaction nobody can reach any more is one the server would hold
// every one of its locks for until the connection died.
func (t *Txn) Close() {
	if t == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.tx == nil {
		return
	}
	t.tx.Rollback()
	t.conn.Close()
	t.tx, t.conn = nil, nil
}

// end finishes the transaction whichever way it was asked to, and gives the connection back either
// way: a commit the server refused has still ended the transaction, and holding the connection on
// after it would leave the session unable to begin another.
func (t *Txn) end(finish func(*sql.Tx) error) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.tx == nil {
		return domain.ErrNoTransaction
	}

	tx, conn := t.tx, t.conn
	t.tx, t.conn = nil, nil

	err := finish(tx)
	conn.Close()
	return t.fail(err)
}

func (t *Txn) fail(err error) error {
	if err == nil || t.Translate == nil {
		return err
	}
	return t.Translate(err)
}
