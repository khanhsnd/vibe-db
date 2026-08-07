package dbtool

import (
	"os"
	"strconv"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

// Request is what the reader asked for, in words that mean the same on every engine. What each
// program *calls* those things is this file's business and nowhere else's — the app says "put the
// rows back and drop what is there first", and pg_restore hears `--clean --if-exists` while
// mongorestore hears `--drop`.
type Request struct {
	Profile  domain.ConnectionProfile
	Password string
	// Database and Schema are the namespace being carried, which is the one the reader was looking
	// at rather than the one the profile happens to open on.
	Database string
	Schema   string
	// Path is the file being written, or the file being read back.
	Path string
	// Include is the tables the reader ticked and Exclude is the ones they did not, both empty for
	// a whole namespace. **Both** are carried because the three programs narrow in two different
	// ways: mysqldump and pg_dump are told what to take, and mongodump only what to leave out.
	Include []string
	Exclude []string
	// Structure and Data are the two halves of a dump, and are asked for only where the engine has
	// two halves — see `domain.Toolkit.SplitsStructure`.
	Structure bool
	Data      bool
	Compress  bool
	// Drop removes what is there before the file is put back, DryRun says what a restore would do
	// without doing it, and StopOnError halts at the first thing the server refuses. Each is
	// offered only where the program has it; see the flags on `domain.Toolkit`.
	Drop        bool
	DryRun      bool
	StopOnError bool
}

// Dump is the command that writes this namespace out to a file.
func Dump(kit Kit, request Request) (Command, error) {
	switch request.Profile.Engine {
	case domain.EngineMySQL:
		return mysqlDump(kit, request), nil
	case domain.EnginePostgres:
		return postgresDump(kit, request), nil
	case domain.EngineMongo:
		return mongoDump(kit, request), nil
	default:
		return Command{}, domain.Unsupported("a dump program of its own")
	}
}

// Restore is the command that puts one of those files back.
func Restore(kit Kit, request Request) (Command, error) {
	switch request.Profile.Engine {
	case domain.EngineMySQL:
		return mysqlRestore(kit, request), nil
	case domain.EnginePostgres:
		return postgresRestore(kit, request), nil
	case domain.EngineMongo:
		return mongoRestore(kit, request)
	default:
		return Command{}, domain.Unsupported("a restore program of its own")
	}
}

// ---------------------------------------------------------------- MySQL

// mysqlDump writes a plain SQL script, which is the only thing mysqldump writes and the reason the
// restore is the ordinary client rather than a program of its own.
//
// `--single-transaction` is what makes the dump one moment of the database rather than a table's
// worth of moments, and it does it without locking anybody out. `--result-file` is what keeps the
// bytes out of this process: the script goes straight from the child to the disk, so a dump of a
// hundred gigabytes costs this app nothing at all.
func mysqlDump(kit Kit, request Request) Command {
	args := append(mysqlConnection(request),
		"--single-transaction",
		"--quick",
		"--result-file="+request.Path,
	)

	if !request.Data {
		args = append(args, "--no-data")
	}
	if !request.Structure {
		args = append(args, "--no-create-info")
	}

	// The database, then whatever tables were ticked — which is how mysqldump is told to narrow,
	// and why an empty list is the whole database rather than nothing.
	args = append(args, request.Database)
	args = append(args, request.Include...)

	return Command{Program: kit.Dump, Args: args, Secret: mysqlSecret(request)}
}

// mysqlRestore runs the script back through the ordinary client, which reads it on **stdin** — the
// `mysql < dump.sql` every MySQL reader already knows, with no shell in the middle to quote a path.
//
// There is no dropping what is there first, and that is not an omission: the client runs a script
// and a script is what the dump wrote. What the file holds is what happens.
func mysqlRestore(kit Kit, request Request) Command {
	args := append(mysqlConnection(request), "--database="+request.Database)

	// The client stops at the first rejection unless it is told otherwise, so the box is the one
	// that keeps it going rather than the one that stops it.
	if !request.StopOnError {
		args = append(args, "--force")
	}

	return Command{
		Program:   kit.Restore,
		Args:      args,
		StdinPath: request.Path,
		Secret:    mysqlSecret(request),
	}
}

// mysqlConnection is the half of the command line that says which server, and is the same for both
// programs — they share the option set and the `[client]` group of the defaults file.
func mysqlConnection(request Request) []string {
	profile := request.Profile

	// `--protocol=TCP` because a host of `localhost` otherwise means a unix socket to these
	// programs, which is a different server from the one this app connected to over TCP.
	args := []string{
		"--host=" + profile.Host,
		"--port=" + strconv.Itoa(profile.Port),
		"--protocol=TCP",
	}
	if profile.User != "" {
		args = append(args, "--user="+profile.User)
	}

	args = append(args, "--ssl-mode="+mysqlSSLMode(profile.TLS.Mode))
	if profile.TLS.Mode.Encrypts() {
		args = appendIf(args, "--ssl-ca=", profile.TLS.RootCert)
		args = appendIf(args, "--ssl-cert=", profile.TLS.ClientCert)
		args = appendIf(args, "--ssl-key=", profile.TLS.ClientKey)
	}
	return args
}

// mysqlSecret is the password, in the one file mysqldump reads before it reads anything else.
// `[client]` rather than `[mysqldump]` because both programs read it and neither reads the other's.
func mysqlSecret(request Request) Secret {
	if request.Password == "" {
		return Secret{}
	}
	return Secret{
		Body:      "[client]\npassword=\"" + mysqlEscape(request.Password) + "\"\n",
		Extension: ".cnf",
		Arg:       "--defaults-file=%s",
		// mysqldump reads this before it parses anything else, and refuses it anywhere but first.
		First: true,
	}
}

// mysqlEscape is a value in a MySQL option file, where a quoted string takes backslash escapes.
func mysqlEscape(value string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value)
}

func mysqlSSLMode(mode domain.TLSMode) string {
	switch mode {
	case domain.TLSDisabled:
		return "DISABLED"
	case domain.TLSRequired:
		return "REQUIRED"
	case domain.TLSVerifyCA:
		return "VERIFY_CA"
	case domain.TLSVerifyFull:
		return "VERIFY_IDENTITY"
	default:
		return "PREFERRED"
	}
}

// ---------------------------------------------------------------- PostgreSQL

// postgresDump writes the **custom** format rather than plain SQL, and that is what makes the pair
// worth having: one file, compressed as it is written, out of which pg_restore can put back the
// whole database, one schema, or one table. A plain script could only ever be run whole.
func postgresDump(kit Kit, request Request) Command {
	args := append(postgresConnection(request),
		"--format=custom",
		"--file="+request.Path,
		// The reader is watching a log, and pg_dump says nothing at all unless it is asked to.
		"--verbose",
	)

	if !request.Data {
		args = append(args, "--schema-only")
	}
	if !request.Structure {
		args = append(args, "--data-only")
	}
	if request.Schema != "" {
		args = append(args, "--schema="+request.Schema)
	}
	for _, table := range request.Include {
		args = append(args, "--table="+postgresQualified(request.Schema, table))
	}

	return Command{Program: kit.Dump, Args: args, Env: postgresEnv(request), Secret: postgresSecret(request)}
}

// postgresRestore puts a custom-format file back. `--clean` is always paired with `--if-exists`:
// on its own it fails on the first object that was not there, which is every object the first time
// a dump is restored onto an empty database.
func postgresRestore(kit Kit, request Request) Command {
	args := append(postgresConnection(request), "--verbose")

	if request.Drop {
		args = append(args, "--clean", "--if-exists")
	}
	if request.StopOnError {
		args = append(args, "--exit-on-error")
	}
	if request.Schema != "" {
		args = append(args, "--schema="+request.Schema)
	}
	for _, table := range request.Include {
		args = append(args, "--table="+table)
	}

	// The file is named last and without a flag, which is the one shape pg_restore takes it in.
	args = append(args, request.Path)

	return Command{Program: kit.Restore, Args: args, Env: postgresEnv(request), Secret: postgresSecret(request)}
}

func postgresConnection(request Request) []string {
	profile := request.Profile

	return []string{
		"--host=" + profile.Host,
		"--port=" + strconv.Itoa(profile.Port),
		"--username=" + profile.User,
		"--dbname=" + request.Database,
		// Never prompt. A child process asking for a password on a terminal nobody is looking at
		// is a dump that hangs for ever; the password is in PGPASSFILE or there is none.
		"--no-password",
	}
}

// postgresEnv is everything these two are told through the environment rather than the command
// line, which for libpq is the whole of TLS.
func postgresEnv(request Request) []string {
	tls := request.Profile.TLS

	env := []string{"PGSSLMODE=" + postgresSSLMode(tls.Mode)}
	if tls.Mode.Encrypts() {
		env = appendIf(env, "PGSSLROOTCERT=", tls.RootCert)
		env = appendIf(env, "PGSSLCERT=", tls.ClientCert)
		env = appendIf(env, "PGSSLKEY=", tls.ClientKey)
	}
	return env
}

// postgresSecret is a `pgpass` file of one line. libpq reads it wherever PGPASSFILE points, which
// is what keeps the password out of both the command line and the environment.
func postgresSecret(request Request) Secret {
	if request.Password == "" {
		return Secret{}
	}

	profile := request.Profile
	line := strings.Join([]string{
		pgpassField(profile.Host),
		strconv.Itoa(profile.Port),
		pgpassField(request.Database),
		pgpassField(profile.User),
		pgpassField(request.Password),
	}, ":")

	return Secret{Body: line + "\n", Extension: ".pgpass", EnvVar: "PGPASSFILE"}
}

// pgpassField escapes the two characters a pgpass line gives a meaning to: the separator, and the
// backslash that escapes it.
func pgpassField(value string) string {
	return strings.NewReplacer(`\`, `\\`, `:`, `\:`).Replace(value)
}

func postgresSSLMode(mode domain.TLSMode) string {
	switch mode {
	case domain.TLSDisabled:
		return "disable"
	case domain.TLSRequired:
		return "require"
	case domain.TLSVerifyCA:
		return "verify-ca"
	case domain.TLSVerifyFull:
		return "verify-full"
	default:
		return "prefer"
	}
}

// postgresQualified is how pg_dump is told about one table: with its schema where the reader was
// looking at one, because two schemas of a database may each hold a table of the same name.
func postgresQualified(schema, table string) string {
	if schema == "" {
		return table
	}
	return schema + "." + table
}

// ---------------------------------------------------------------- MongoDB

// mongoDump writes **one archive file** rather than a directory of BSON, because a backup that is
// one file is a backup a reader can move, and because the directory form would put this app in the
// business of naming folders on the reader's behalf.
//
// A collection is narrowed by what is *left out* rather than by what is taken: mongodump takes one
// `--collection` and any number of `--excludeCollection`, so the picker's unticked list is what it
// is told — which is also exactly what the picker remembers.
func mongoDump(kit Kit, request Request) Command {
	args := append(mongoConnection(request),
		"--db="+request.Database,
		"--archive="+request.Path,
	)

	if request.Compress {
		args = append(args, "--gzip")
	}
	for _, collection := range request.Exclude {
		args = append(args, "--excludeCollection="+collection)
	}

	return Command{Program: kit.Dump, Args: args, Secret: mongoSecret(request)}
}

// mongoRestore puts an archive back.
//
// **Whether the archive is compressed is read off the file, not off a box.** `--gzip` has to match
// how the dump was written or mongorestore refuses the first byte, and a reader restoring a file
// from last month has no reason to remember which it was — so the first two bytes are looked at,
// which is the one place that answer is written down.
//
// There is no `--db`: an archive carries the namespace it came from, and the documents go back
// where they came from however the tree is pointed. Renaming one on the way in is `--nsFrom`, which
// this does not offer, because a restore that silently landed somewhere else would be worse than
// one that says where it went.
func mongoRestore(kit Kit, request Request) (Command, error) {
	args := mongoConnection(request)

	compressed, err := gzipped(request.Path)
	if err != nil {
		return Command{}, err
	}

	args = append(args, "--archive="+request.Path)
	if compressed {
		args = append(args, "--gzip")
	}
	if request.Drop {
		args = append(args, "--drop")
	}
	if request.DryRun {
		args = append(args, "--dryRun")
	}
	if request.StopOnError {
		args = append(args, "--stopOnError")
	}
	for _, collection := range request.Include {
		args = append(args, "--nsInclude="+request.Database+"."+collection)
	}

	return Command{Program: kit.Restore, Args: args, Secret: mongoSecret(request)}, nil
}

func mongoConnection(request Request) []string {
	profile := request.Profile

	args := []string{
		"--host=" + profile.Host,
		"--port=" + strconv.Itoa(profile.Port),
	}

	if profile.User != "" {
		// Without this, these programs authenticate against the database being dumped, while this
		// app's own driver authenticates against `admin`. Two different logins for one profile is
		// the kind of difference that only shows up on the server that has both.
		args = append(args, "--username="+profile.User, "--authenticationDatabase=admin")
	}

	if profile.TLS.Mode.Encrypts() {
		args = append(args, "--ssl")
		args = appendIf(args, "--sslCAFile=", profile.TLS.RootCert)
		args = appendIf(args, "--sslPEMKeyFile=", profile.TLS.ClientCert)
		if !profile.TLS.Mode.Verifies() {
			// Required encrypts and checks nothing, which is what this pair of flags is on the
			// engine's own tools — the same reading the driver gives the same mode.
			args = append(args, "--sslAllowInvalidCertificates", "--sslAllowInvalidHostnames")
		}
	}
	return args
}

// mongoSecret is the password, in the YAML file these programs read exactly three fields out of.
func mongoSecret(request Request) Secret {
	if request.Password == "" {
		return Secret{}
	}
	return Secret{
		Body:      "password: " + mongoYAML(request.Password) + "\n",
		Extension: ".yaml",
		Arg:       "--config=%s",
	}
}

// mongoYAML writes one scalar as a double-quoted YAML string, which is the one form that carries a
// password holding a `#`, a `:` or a leading digit without changing what it is.
func mongoYAML(value string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value) + `"`
}

// gzipped says whether a file starts with the two bytes every gzip stream starts with. A file too
// short to hold them is not one, which is the same answer mongorestore would reach.
func gzipped(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, domain.FileError{Err: err}
	}
	defer file.Close()

	header := make([]byte, 2)
	read, err := file.Read(header)
	if err != nil && read < 2 {
		// A file that cannot be read past its first byte is one the program is about to complain
		// about in words of its own, which are better than any this could invent.
		return false, nil
	}
	return header[0] == 0x1f && header[1] == 0x8b, nil
}

// appendIf adds `flag=value` where there is a value, which is what keeps an empty certificate path
// from being sent as a real one.
func appendIf(args []string, flag, value string) []string {
	if strings.TrimSpace(value) == "" {
		return args
	}
	return append(args, flag+value)
}
