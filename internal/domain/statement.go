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

// ImportMode is what an import does to the rows a table already holds. It is a domain type rather
// than a wire one for the reason the two grammars below are: what a mode is going to do decides
// whether it has to be answered for before it runs, and that decision is one rule wherever it is
// asked — a typed statement, a shell command, or a wizard.
type ImportMode string

const (
	// ImportAppend adds every row of the file and changes none of the table's.
	ImportAppend ImportMode = "append"
	// ImportUpdate changes the rows the key names, and adds none.
	ImportUpdate ImportMode = "update"
	// ImportUpsert changes the rows the key names and adds the ones it does not.
	ImportUpsert ImportMode = "upsert"
	// ImportSkip adds a row the key does not already name and leaves the ones it does alone.
	ImportSkip ImportMode = "skip"
	// ImportDelete removes the rows the key names.
	ImportDelete ImportMode = "delete"
	// ImportCopy empties the table and then appends, which is the one mode that destroys rows the
	// file says nothing about.
	ImportCopy ImportMode = "copy"
)

// NeedsKey says this mode finds stored rows before it does anything to them, and so cannot run
// without being told which columns identify one. Appending needs none: it names nothing.
func (m ImportMode) NeedsKey() bool {
	return m == ImportUpdate || m == ImportUpsert || m == ImportSkip || m == ImportDelete
}

// ImportHazard is StatementHazard for an import — the third grammar for one rule, and here for the
// reason MongoHazard is: what is dangerous to run is a domain rule and only its spelling changes.
//
// Copy is destructive: it takes every row the table held, whether or not the file mentions it, which
// is a TRUNCATE by another name. A mode that reaches for rows *by* a key and was given none is
// unfiltered in exactly the sense a `DELETE` with no `WHERE` is — it names no row and so names all
// of them — and is answered for on a laptop as well as on production.
func ImportHazard(mode ImportMode, keyed bool) Hazard {
	switch {
	case mode == ImportCopy:
		return HazardDestructive
	case mode.NeedsKey() && !keyed:
		return HazardUnfiltered
	default:
		return HazardWrite
	}
}

// AccountAction is what a change to an account does, in the words the driver's own change kinds
// use. It is a domain type for the reason ImportMode is: what a change is going to do decides
// whether it has to be answered for before it runs, and the driver spells it rather than deciding
// it.
type AccountAction string

const (
	AccountCreate AccountAction = "create"
	AccountAlter  AccountAction = "alter"
	AccountRename AccountAction = "rename"
	AccountDrop   AccountAction = "drop"
	AccountGrant  AccountAction = "grant"
	AccountRevoke AccountAction = "revoke"
)

// GrantScope is how wide a grant reaches, which is the only thing about *where* a privilege lands
// that changes whether it is answered for. A driver's own levels below a server all collapse into
// GrantScoped: what matters here is whether the grant named something.
type GrantScope string

const (
	// GrantWhole is the whole server — MySQL's `*.*`, an Oracle system privilege, a SQL Server
	// server permission. It is the grant that named nothing.
	GrantWhole GrantScope = "whole"
	// GrantScoped is a grant that named a database, a schema, a table or a column.
	GrantScoped GrantScope = "scoped"
)

// grantAll is every spelling of "all of them" the five SQL engines use. A grant of one of these is
// treated as reaching the whole of whatever it was granted on, however narrow that was: `ALL
// PRIVILEGES ON one_table` still hands over the right to drop it.
var grantAll = map[string]bool{
	"all":             true,
	"all privileges":  true,
	"control":         true,
	"control server":  true,
	"superuser":       true,
	"sysadmin":        true,
	"grant option":    true,
	"dba":             true,
	"alter any table": true,
}

// GrantHazard is StatementHazard for a change to an account — the fifth grammar for one rule, and
// here for the reason MongoHazard and RedisHazard are: what is dangerous to run is a domain rule,
// and only its spelling is the engine's.
//
// Three answers, and the middle one is the point:
//
//   - **Dropping an account is destructive**, in the same class a DROP is. Nothing puts back what
//     the account held, and a role dropped takes every grant that came through it with it.
//   - **A grant that names nothing is unfiltered**, in exactly the sense a DELETE with no WHERE is:
//     `GRANT ALL PRIVILEGES ON *.*` names no object and so names all of them. So is a grant of
//     everything however narrowly scoped, and so is one handing on the right to grant — an account
//     that can grant can grant itself. Environment.NeedsConfirmation answers that class **on a
//     laptop as well as on production**, which is the whole reason this is not merely a write.
//   - **Everything else that changes an account is a write**, including a revoke: taking a
//     privilege away breaks whatever was using it, but it hands nothing to anybody.
func GrantHazard(action AccountAction, scope GrantScope, privilege string, grantable bool) Hazard {
	if action == AccountDrop {
		return HazardDestructive
	}
	if action != AccountGrant {
		return HazardWrite
	}
	if scope == GrantWhole || grantable || grantAll[strings.ToLower(strings.TrimSpace(privilege))] {
		return HazardUnfiltered
	}
	return HazardWrite
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

// redisHazards is the command against what it will do, which is where a key-value store keeps the
// verb the other two grammars keep in a first word and in a method. A command that is not here
// reads, and so does everything this side cannot tell apart from a read.
//
// A **container** command carries its verb in its second word — `CONFIG SET` writes and `CONFIG GET`
// does not — so those are keyed by both words. RedisHazard tries the pair before the name, which is
// what keeps `CLIENT LIST` out of the same class as `CLIENT KILL`.
var redisHazards = map[string]Hazard{
	// Everything that names the keys it touches. A DEL is a DELETE that carried its WHERE.
	"del": HazardWrite, "unlink": HazardWrite, "set": HazardWrite, "setnx": HazardWrite,
	"setex": HazardWrite, "psetex": HazardWrite, "getset": HazardWrite, "getdel": HazardWrite,
	"getex": HazardWrite, "mset": HazardWrite, "msetnx": HazardWrite, "append": HazardWrite,
	"setrange": HazardWrite, "setbit": HazardWrite, "bitop": HazardWrite, "bitfield": HazardWrite,
	"incr": HazardWrite, "incrby": HazardWrite, "incrbyfloat": HazardWrite, "decr": HazardWrite,
	"decrby": HazardWrite, "copy": HazardWrite, "restore": HazardWrite, "move": HazardWrite,
	"expire": HazardWrite, "expireat": HazardWrite, "pexpire": HazardWrite, "pexpireat": HazardWrite,
	"persist": HazardWrite,
	"hset":    HazardWrite, "hsetnx": HazardWrite, "hdel": HazardWrite, "hincrby": HazardWrite,
	"hincrbyfloat": HazardWrite, "hexpire": HazardWrite,
	"lpush": HazardWrite, "rpush": HazardWrite, "lpushx": HazardWrite, "rpushx": HazardWrite,
	"lpop": HazardWrite, "rpop": HazardWrite, "lmpop": HazardWrite, "blpop": HazardWrite,
	"brpop": HazardWrite, "lset": HazardWrite, "linsert": HazardWrite, "lrem": HazardWrite,
	"ltrim": HazardWrite, "lmove": HazardWrite, "rpoplpush": HazardWrite,
	"sadd": HazardWrite, "srem": HazardWrite, "spop": HazardWrite, "smove": HazardWrite,
	"sinterstore": HazardWrite, "sunionstore": HazardWrite, "sdiffstore": HazardWrite,
	"zadd": HazardWrite, "zrem": HazardWrite, "zincrby": HazardWrite, "zpopmin": HazardWrite,
	"zpopmax": HazardWrite, "zremrangebyrank": HazardWrite, "zremrangebyscore": HazardWrite,
	"zremrangebylex": HazardWrite, "zrangestore": HazardWrite, "zdiffstore": HazardWrite,
	"zunionstore": HazardWrite, "zinterstore": HazardWrite,
	"xadd": HazardWrite, "xdel": HazardWrite, "xtrim": HazardWrite, "xsetid": HazardWrite,
	"xack": HazardWrite, "xclaim": HazardWrite, "xautoclaim": HazardWrite,
	"pfadd": HazardWrite, "pfmerge": HazardWrite, "pfcount": HazardWrite,
	"geoadd": HazardWrite, "geosearchstore": HazardWrite, "georadius": HazardWrite,
	"json.set": HazardWrite, "json.del": HazardWrite, "json.arrappend": HazardWrite,
	// A script is whatever it was written to be, which is `runCommand`'s position exactly: it is
	// answered for because only the server knows what it will do. The `_RO` forms cannot write.
	"eval": HazardWrite, "evalsha": HazardWrite, "fcall": HazardWrite,
	"xgroup create": HazardWrite, "xgroup destroy": HazardDestructive,
	"acl setuser": HazardWrite,

	// Renaming a key is the same class RENAME is in every other grammar: what was there is gone
	// under that name and no transaction puts it back.
	"rename": HazardDestructive, "renamenx": HazardDestructive, "migrate": HazardDestructive,
	// The server rather than its keys. A CONFIG SET outlives the connection that made it.
	"shutdown": HazardDestructive, "failover": HazardDestructive, "replicaof": HazardDestructive,
	"slaveof": HazardDestructive, "debug": HazardDestructive,
	"config set": HazardDestructive, "config rewrite": HazardDestructive,
	"config resetstat": HazardDestructive, "client kill": HazardDestructive,
	"cluster forget": HazardDestructive, "acl deluser": HazardDestructive,
}

// redisSweeping is every command that names nothing and therefore takes everything. They are
// unfiltered in exactly the sense a `DELETE` with no `WHERE` is — which is why they are answered
// for on a laptop as well as on production, and why they are not merely destructive: a FLUSHALL
// nobody meant is the one mistake this class exists to catch.
var redisSweeping = map[string]bool{
	"flushdb":        true,
	"flushall":       true,
	"swapdb":         true,
	"script flush":   true,
	"function flush": true,
	"cluster reset":  true,
}

// RedisHazard is StatementHazard for a key-value store: the command is the verb, and a container
// command's second word is part of it. It is the same rule read off a third grammar, and it lives
// here beside the other two for the reason they do — what is dangerous to run is a domain rule, and
// only its spelling is the engine's.
func RedisHazard(command, argument string) Hazard {
	name := strings.ToLower(strings.TrimSpace(command))
	pair := name + " " + strings.ToLower(strings.TrimSpace(argument))

	switch {
	case redisSweeping[pair], redisSweeping[name]:
		return HazardUnfiltered
	default:
		if hazard, ok := redisHazards[pair]; ok {
			return hazard
		}
		return redisHazards[name]
	}
}
