package objectstore

import (
	"path/filepath"
	"strings"
)

// LocalPath is where an object's bytes are written on this machine, and it is the one function in
// this package that has to be *hostile* about its input.
//
// A key is a string a stranger chose. `../../.ssh/authorized_keys` is a perfectly legal key, so is
// `C:\Windows\System32\drivers\etc\hosts`, and so is a key that is nothing but dots. A downloader
// that pasted a key onto a path would be a downloader that writes wherever the bucket's owner
// likes — which is the whole class of bug this exists to close. **Every segment goes through
// `segment`, and the result is checked to be under the root even so**: two guards, because the
// first one being wrong is not a thing anybody would notice until it mattered.
//
// `base` is the prefix the reader was standing in, and is taken off the front so that downloading
// `logs/2024/` puts `2024/03/a.log` in `<root>/2024/03/a.log` rather than under a `logs` nobody
// asked for. It is the same rule an archiver follows about the folder it was pointed at.
func LocalPath(root, base, key string) (string, bool) {
	relative := strings.TrimPrefix(key, base)
	relative = strings.TrimPrefix(relative, Separator)
	if relative == "" {
		return "", false
	}

	parts := make([]string, 0, 8)
	for _, part := range strings.Split(relative, Separator) {
		clean := segment(part)
		if clean == "" {
			continue
		}
		parts = append(parts, clean)
	}
	if len(parts) == 0 {
		return "", false
	}

	path := filepath.Join(append([]string{root}, parts...)...)

	// The second guard. `segment` already refuses everything that could climb, so this can only
	// fire if it is wrong — which is exactly when it is worth having.
	inside, err := filepath.Rel(root, path)
	if err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
		return "", false
	}
	return path, true
}

// reserved is every name Windows will not let a file have, whatever its extension. A key called
// `con.log` is a key, and a file called `con.log` is a device — so it is written as `con_.log`
// rather than failing at the last moment with an error the reader cannot act on.
var reserved = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// segment is one piece of a key made safe to be one piece of a path. It is `internal/backup`'s own
// `segment` applied to somebody else's string rather than to the reader's: the same job, and a
// stricter caller.
//
// What it answers with is **empty** for a segment that names a place rather than a thing — `.`,
// `..`, a drive letter, an empty piece — because such a segment is dropped rather than repaired: a
// `..` turned into `__` would be a folder appearing in somebody's download for no reason.
func segment(part string) string {
	trimmed := strings.TrimSpace(part)
	if trimmed == "" || trimmed == "." || trimmed == ".." {
		return ""
	}
	// A Windows drive letter or a UNC root reaching a path as one *segment* is the case
	// `filepath.Join` would quietly honour, so it goes here rather than in the guard below.
	if len(trimmed) == 2 && trimmed[1] == ':' {
		return ""
	}

	var out strings.Builder
	for _, character := range trimmed {
		switch {
		case character < 0x20, character == 0x7f:
			// A control character in a file name is a file name no tool on this machine can quote.
			out.WriteRune('_')
		case strings.ContainsRune(`<>:"/\|?*`, character):
			out.WriteRune('_')
		default:
			out.WriteRune(character)
		}
	}

	// A name ending in a dot or a space is a name Windows silently trims, so two keys that differ
	// only in that would land on one file. Trimming here is what makes the two land on the same
	// file *deliberately*, where the reader's own overwrite rule then decides — see `Collision`.
	name := strings.TrimRight(out.String(), ". ")
	if name == "" {
		return "_"
	}

	stem := name
	if dot := strings.Index(name, "."); dot > 0 {
		stem = name[:dot]
	}
	if reserved[strings.ToLower(stem)] {
		name = stem + "_" + name[len(stem):]
	}
	return name
}
