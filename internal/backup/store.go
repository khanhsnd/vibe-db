// Package backup keeps a namespace's backups on this machine.
//
// A backup is **a file of statements under a header line** — the same statements a dump writes, so
// restoring one is running a file and nothing here has to know how any engine spells anything. The
// header is inside the file rather than in an index beside it, which is what makes a backup one
// thing: copied to another machine, moved between folders or written by an older build, it still
// says which server it came off and what the reader wrote about it.
//
// Nothing here talks to a database. What goes into the file is the driver's and what runs it back
// is the session's; this package is only where the file lives and what its first line says.
package backup

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/jsonstore"
)

const (
	extension = ".sql"

	// marker opens a backup's first line. It is a SQL comment, so a file carrying a header is still
	// an ordinary script: anything that can run the statements can run the whole file unchanged.
	marker = "-- vibe-db-backup "

	// headerLimit is how far into a file the header is looked for. A backup's second line may be a
	// single insert of a million values, and reading a listing must not pull one into memory.
	headerLimit = 64 << 10

	dirMode  = 0o700
	fileMode = 0o600

	// maxNameAttempts bounds the suffix search in Create. Past it the folder is answering that a
	// file is there and cannot be made, which is a disk to look at rather than a name to try again.
	maxNameAttempts = 16
)

// NameLayout is what a backup is called: the moment it was taken, to the second. A name that is a
// timestamp sorts by age and by name at once, which is why the list needs no second key.
const NameLayout = "20060102150405"

// errNoHeader is a file in the folder that this app did not write, or wrote before the header
// existed. It is not a failure the reader can act on — the file still restores — so a listing
// swallows it and shows the row with no comment.
var errNoHeader = errors.New("no backup header")

// Target is whose backups these are: one namespace of one connection. It keeps the profile's **id**
// rather than its name, because a connection renamed is the same connection and its backups do not
// move when the reader retitles it.
type Target struct {
	ProfileID string
	Database  string
	Schema    string
}

// Header is what a backup says about itself, as JSON inside the comment on its first line.
type Header struct {
	// Server is the connection it was taken off, in the words the reader knows it by, so a file
	// found on disk names its origin without the app having to still hold that profile.
	Server   string    `json:"server"`
	Database string    `json:"database"`
	Created  time.Time `json:"created"`
	Comment  string    `json:"comment"`
}

// File is one backup as the list draws a row for it.
type File struct {
	Name     string
	Path     string
	Modified time.Time
	Size     int64
	Comment  string
}

// Store is the folder every backup lives under.
type Store struct {
	root string
}

func New(root string) *Store {
	return &Store{root: root}
}

// DefaultRoot is one directory beside the connection profiles and the query history, so everything
// this app keeps for the reader sits in one place.
func DefaultRoot() (string, error) {
	return jsonstore.Path("backups")
}

// Root is the folder every namespace's backups sit under, which is what Open Containing Folder
// falls back to for a namespace nothing has been written for yet.
func (s *Store) Root() string {
	return s.root
}

// Folder is where one namespace's backups live, made if it is not there yet. It is made rather than
// only named because every caller either writes into it, lists it or shows it to the reader, and a
// folder that is not there reads as a failure in all three.
func (s *Store) Folder(target Target) (string, error) {
	dir := filepath.Join(s.root, segment(target.ProfileID), segment(target.key()))
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return "", domain.FileError{Err: err}
	}
	return dir, nil
}

// Path is where the backup called name lives. The name is checked to be one path segment: a listing
// hands its own names straight back here, and a name carrying a separator would otherwise address a
// file outside the folder it was listed from.
func (s *Store) Path(target Target, name string) (string, error) {
	if name == "" || name != segment(name) || filepath.Base(name) != name {
		return "", domain.NameError{What: "backup"}
	}

	dir, err := s.Folder(target)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+extension), nil
}

// Create opens a new backup for writing and puts its header on the first line, so a file that
// exists at all is a file that says what it is.
//
// It **never writes over one that is there**, which is what `O_EXCL` is for: a name is a moment to
// the second, and two backups taken inside one second — a double-clicked button — would otherwise be
// one file with the second one's contents. The second is given a suffix instead.
func (s *Store) Create(target Target, name string, header Header) (*os.File, error) {
	for attempt := 1; ; attempt++ {
		candidate := name
		if attempt > 1 {
			candidate = name + "-" + strconv.Itoa(attempt)
		}

		path, err := s.Path(target, candidate)
		if err != nil {
			return nil, err
		}

		out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fileMode)
		if errors.Is(err, fs.ErrExist) && attempt < maxNameAttempts {
			continue
		}
		if err != nil {
			return nil, domain.FileError{Err: err}
		}

		if err := writeHeader(out, header); err != nil {
			out.Close()
			os.Remove(path)
			return nil, err
		}
		return out, nil
	}
}

// Name is what a backup at this path is called: its file name without the extension, which is what
// every other method here takes. It is read off the path rather than remembered, because `Create`
// is allowed to have chosen a different one — see the suffix above.
func Name(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// List is every backup this target holds, newest first. A file whose header cannot be read is still
// listed — it restores exactly as the others do — with an empty comment rather than being hidden.
func (s *Store) List(target Target) ([]File, error) {
	dir, err := s.Folder(target)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, domain.FileError{Err: err}
	}

	files := make([]File, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), extension) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		header, _ := ReadHeader(path)

		files = append(files, File{
			Name:     strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())),
			Path:     path,
			Modified: info.ModTime(),
			Size:     info.Size(),
			Comment:  header.Comment,
		})
	}

	sort.Slice(files, func(one, other int) bool {
		return files[one].Modified.After(files[other].Modified)
	})
	return files, nil
}

// Copy duplicates one backup beside itself and answers with what the copy is called. The name is
// the original's with `-copy` after it, and a number after that where one is already there — the
// same free-name search Create does, and for the same reason: nothing here writes over a backup.
func (s *Store) Copy(target Target, name string) (string, error) {
	source, err := s.Path(target, name)
	if err != nil {
		return "", err
	}

	for attempt := 1; attempt <= maxNameAttempts; attempt++ {
		candidate := name + "-copy"
		if attempt > 1 {
			candidate += "-" + strconv.Itoa(attempt)
		}

		path, err := s.Path(target, candidate)
		if err != nil {
			return "", err
		}

		out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fileMode)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", domain.FileError{Err: err}
		}

		if err := copyInto(out, source); err != nil {
			out.Close()
			os.Remove(path)
			return "", err
		}
		return candidate, nil
	}
	return "", domain.FileError{Err: fs.ErrExist}
}

// Rename gives one backup another name. It **refuses a name already taken** rather than writing
// over it: the two files are two backups, and the reader asked to move one, not to lose the other.
func (s *Store) Rename(target Target, name, to string) error {
	from, err := s.Path(target, name)
	if err != nil {
		return err
	}
	path, err := s.Path(target, to)
	if err != nil {
		return err
	}
	if path == from {
		return nil
	}

	// A ValidationError rather than a NameError: the name is not *missing*, which is the only thing
	// a NameError says, and the reader's next move is to type a different one in the box they are
	// already in.
	if _, err := os.Stat(path); err == nil {
		return domain.ValidationError{Field: "name", Message: "is already taken by another backup"}
	}
	if err := os.Rename(from, path); err != nil {
		return domain.FileError{Err: err}
	}
	return nil
}

// Remove deletes one backup. There is no undoing it, which is why what asks for it asks twice.
func (s *Store) Remove(target Target, name string) error {
	path, err := s.Path(target, name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return domain.FileError{Err: err}
	}
	return nil
}

// ReadHeader reads a backup's first line and nothing else — see headerLimit for why the rest of the
// file is never touched by a listing.
func ReadHeader(path string) (Header, error) {
	file, err := os.Open(path)
	if err != nil {
		return Header{}, domain.FileError{Err: err}
	}
	defer file.Close()

	line, err := bufio.NewReader(io.LimitReader(file, headerLimit)).ReadString('\n')
	if err != nil && line == "" {
		return Header{}, errNoHeader
	}
	if !strings.HasPrefix(line, marker) {
		return Header{}, errNoHeader
	}

	var header Header
	if err := json.Unmarshal([]byte(strings.TrimSpace(line[len(marker):])), &header); err != nil {
		return Header{}, errNoHeader
	}
	return header, nil
}

// copyInto writes the whole of one backup into an already-open file, and answers for the close as
// well as deferring it: a buffered write reports nothing about the disk until then.
func copyInto(out *os.File, source string) error {
	in, err := os.Open(source)
	if err != nil {
		return domain.FileError{Err: err}
	}
	defer in.Close()

	if _, err := io.Copy(out, in); err != nil {
		return domain.FileError{Err: err}
	}
	if err := out.Close(); err != nil {
		return domain.FileError{Err: err}
	}
	return nil
}

func writeHeader(out io.Writer, header Header) error {
	encoded, err := json.Marshal(header)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "%s%s\n\n", marker, encoded); err != nil {
		return domain.FileError{Err: err}
	}
	return nil
}

// key is the folder one namespace's backups sit in: the database, and the schema after it where the
// engine has them, so two schemas of one database keep their backups apart.
func (t Target) key() string {
	if t.Schema == "" {
		return t.Database
	}
	return t.Database + "@" + t.Schema
}

// segment is one name made safe to be one directory. A character a path may not hold is replaced
// rather than dropped, so two databases differing only in punctuation do not share a folder, and an
// empty name still occupies a level of its own rather than collapsing into its parent.
//
// `.` and `..` are names a path *may* hold and neither is a directory: joined to the root they walk
// out of it, which is the one way a name could address a folder that is not ours.
func segment(name string) string {
	if name == "" || name == "." || name == ".." {
		return "_"
	}
	return strings.Map(func(char rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, char) || char < 0x20 {
			return '_'
		}
		return char
	}, name)
}
