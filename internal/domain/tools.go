package domain

// Toolkit is the pair of programs an engine's own vendor ships for carrying a whole database out to
// a file and putting it back: mongodump and mongorestore, mysqldump and mysql, pg_dump and
// pg_restore. They are not this app's, they are not on this machine unless the reader put them
// there, and what they write is **the engine's own format** rather than a file of statements —
// which is the whole reason they are worth running at all.
//
// That is the difference between this and a [backup]: a backup is a script this app spelled, and a
// tool dump is the vendor's own bytes. A MongoDB collection is the case that proves it. A script of
// this app's would carry the fields a sample of two hundred documents happened to hold; mongodump
// carries the BSON, every field of every document, the indexes and the collection options with it.
//
// A blank here is an engine whose vendor ships no such pair a client can run: SQL Server's is a
// server-side backup to a path the *server* can see, Oracle's Data Pump runs inside the instance,
// and a SQLite database is one file — copying it is the dump. Each answers Unsupported naming
// itself rather than running something that could only half work.
type Toolkit struct {
	// Dump and Restore are what the two programs are *called*, which is also what the menu says:
	// a reader who has installed MongoDB Database Tools is looking for the word `mongodump`.
	Dump    string
	Restore string
	// Extension is what the file this pair writes is called, and Format is how that reads on a
	// picker: `archive` is not a script and `.dump` is not SQL, so neither is offered as one.
	Extension string
	Format    string

	// SplitsStructure says the dump can be asked for the shape alone or the rows alone. A document
	// store cannot: a collection declares nothing, so there is no half of it to leave out.
	SplitsStructure bool
	// Drops says the restore can remove what is there before it puts the file back. Where it is
	// false — the mysql client, which only runs a script — a restore lands *beside* what the
	// database already holds, and the dialog says so rather than offering a box that does nothing.
	Drops bool
	// DryRuns says the restore can be asked what it would do without doing it.
	DryRuns bool
	// Compresses says the dump can be asked to compress as it writes. pg_dump's custom format
	// already does, which is why it is false there rather than a box that is on and cannot move.
	Compresses bool
}

// Toolkit is the pair this engine's vendor ships, and whether there is one at all.
func (e Engine) Toolkit() (Toolkit, bool) {
	switch e {
	case EngineMySQL:
		return Toolkit{
			Dump:            "mysqldump",
			Restore:         "mysql",
			Extension:       "sql",
			Format:          "SQL script",
			SplitsStructure: true,
		}, true

	case EnginePostgres:
		// The custom format rather than plain SQL, because it is what makes pg_restore selective:
		// one file restores a whole database, one schema or one table out of it. It is compressed
		// already, which is why Compresses is false.
		return Toolkit{
			Dump:            "pg_dump",
			Restore:         "pg_restore",
			Extension:       "dump",
			Format:          "PostgreSQL archive",
			SplitsStructure: true,
			Drops:           true,
		}, true

	case EngineMongo:
		return Toolkit{
			Dump:       "mongodump",
			Restore:    "mongorestore",
			Extension:  "archive",
			Format:     "MongoDB archive",
			Drops:      true,
			DryRuns:    true,
			Compresses: true,
		}, true

	default:
		return Toolkit{}, false
	}
}

// HasToolkit is the rule the tree asks before it draws the two items, and the frontend receives its
// answer as `dumpTool` rather than comparing an engine's name.
func (e Engine) HasToolkit() bool {
	_, ok := e.Toolkit()
	return ok
}
