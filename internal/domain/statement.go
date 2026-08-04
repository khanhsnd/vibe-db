package domain

import "strings"

// Hazard is how much of what a statement does cannot be taken back. It is read off the statement
// itself and knows nothing about where it is going to run: pairing it with an Environment is
// Environment.NeedsConfirmation's business, so the same statement can be waved through on a
// laptop and answered for on production without being classified twice.
type Hazard string

const (
	// HazardNone reads and changes nothing: a SELECT, an EXPLAIN, a SHOW.
	HazardNone Hazard = ""
	// HazardWrite changes rows the statement itself has named — an INSERT, or an UPDATE or a
	// DELETE that carries a WHERE.
	HazardWrite Hazard = "write"
	// HazardUnfiltered is an UPDATE or a DELETE with no WHERE at all, which is every row of the
	// table whether or not that was the intention.
	HazardUnfiltered Hazard = "unfiltered"
	// HazardDestructive takes the object rather than its rows: a DROP, a TRUNCATE, an ALTER.
	// No transaction puts a dropped table back on MySQL, which is why these are their own class.
	HazardDestructive Hazard = "destructive"
)

// hazards is the leading keyword of a statement against what it will do. A keyword that is not
// here does nothing worth stopping a reader over — every read is in that silence, and so is
// everything the app cannot tell apart from one.
//
// `update` and `delete` are absent because their answer depends on a clause rather than only on
// the keyword; StatementHazard is where the two are decided.
var hazards = map[string]Hazard{
	"drop":     HazardDestructive,
	"truncate": HazardDestructive,
	"alter":    HazardDestructive,
	"rename":   HazardDestructive,
	"insert":   HazardWrite,
	"replace":  HazardWrite,
	"merge":    HazardWrite,
	"upsert":   HazardWrite,
	"create":   HazardWrite,
	"grant":    HazardWrite,
	"revoke":   HazardWrite,
	"call":     HazardWrite,
	"do":       HazardWrite,
	"vacuum":   HazardWrite,
	"reindex":  HazardWrite,
}

// StatementHazard reads what a statement is going to do from its leading keyword and, for the two
// where the keyword is not enough, whether it carries a WHERE. Both facts are the caller's to
// supply because both are lexical and therefore the engine's: `sqlkit.LeadingKeyword` and
// `Dialect.HasClause` are what read them, and this is the rule they are read for.
func StatementHazard(keyword string, filtered bool) Hazard {
	switch keyword {
	case "update", "delete":
		if filtered {
			return HazardWrite
		}
		return HazardUnfiltered
	default:
		return hazards[keyword]
	}
}

// mongoHazards is the *method* of a shell command against what it will do, which is where a
// document store keeps the verb SQL keeps in its first word. A method that is not here reads,
// and so does everything this side cannot tell apart from a read.
//
// The names come in two spellings on purpose. A reader writes the shell's — `insertOne`, `drop` —
// and a reader reaching past it writes the *server's*, which `db.runCommand({ create: … })` carries
// as the first field of its argument. Both are answered for, because both do the same thing.
var mongoHazards = map[string]Hazard{
	"drop":              HazardDestructive,
	"dropdatabase":      HazardDestructive,
	"dropindex":         HazardDestructive,
	"dropindexes":       HazardDestructive,
	"renamecollection":  HazardDestructive,
	"converttocapped":   HazardDestructive,
	"insertone":         HazardWrite,
	"insertmany":        HazardWrite,
	"insert":            HazardWrite,
	"bulkwrite":         HazardWrite,
	"createcollection":  HazardWrite,
	"create":            HazardWrite,
	"createindex":       HazardWrite,
	"createindexes":     HazardWrite,
	"createview":        HazardWrite,
	"collmod":           HazardWrite,
	"findoneandreplace": HazardWrite,
	// A command is whatever it is told to be, so it is answered for wherever that matters:
	// `runCommand` can drop a database as easily as it can read a build's version.
	"runcommand": HazardWrite,
}

// mongoFiltered is every method whose reach is its filter's rather than its name's: given an
// empty filter each of them takes every document in the collection. It is the counterpart of `update`
// and `delete` being absent from the SQL list above, and it carries the server's own spellings for
// the same reason mongoHazards does.
var mongoFiltered = map[string]bool{
	"updateone":        true,
	"updatemany":       true,
	"update":           true,
	"replaceone":       true,
	"deleteone":        true,
	"deletemany":       true,
	"delete":           true,
	"findoneandupdate": true,
	"findoneanddelete": true,
}

// MongoHazard is StatementHazard for a document store: the method is the verb, and `filtered`
// says the command narrowed itself to something less than the whole collection. It is the same
// rule read off a different grammar, which is why it lives here beside the SQL one rather than in
// the adapter — what is dangerous to run is a domain rule, and only its spelling is the engine's.
func MongoHazard(method string, filtered bool) Hazard {
	name := strings.ToLower(method)
	if mongoFiltered[name] {
		if filtered {
			return HazardWrite
		}
		return HazardUnfiltered
	}
	return mongoHazards[name]
}
