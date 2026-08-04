package domain

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
