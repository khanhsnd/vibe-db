package app

import (
	"os"

	"github.com/khanhsnd/vibe-db/internal/fonts"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// SystemAPI answers what the host machine offers rather than what a server does: the font families
// Settings picks from, and a path out of the host's own file picker. It holds no state and needs no
// session — which is exactly why the picker lives here, since the boxes that use it are filled in
// before there is a connection to fill them for.
type SystemAPI struct {
	apiContext
}

func NewSystemAPI() *SystemAPI {
	return &SystemAPI{apiContext: newAPIContext()}
}

// ListFonts answers the font families installed on this machine, sorted. It returns no error on
// purpose: a machine whose font list cannot be read is not a failure a reader can act on, and the
// picker falls back to the families the stylesheet ships with.
func (s *SystemAPI) ListFonts() []string {
	return fonts.Families()
}

// ChooseFile raises the host's own file picker and answers with the path it was given. An empty path
// is a dismissed dialog rather than a failure, exactly as ChooseScriptFile reads.
//
// The title says what is being picked and doubles as the name of the filter `pattern` is offered
// under; All files sits behind it always, and is the whole list for an empty pattern — because a
// private key or a known_hosts file usually carries no extension at all.
//
// Nothing is read here. What is at the path is opened at connect time and never by this.
func (s *SystemAPI) ChooseFile(title, pattern string) (string, error) {
	filters := []wailsruntime.FileFilter{}
	if pattern != "" {
		filters = append(filters, wailsruntime.FileFilter{DisplayName: title, Pattern: pattern})
	}
	filters = append(filters, wailsruntime.FileFilter{DisplayName: "All files", Pattern: "*.*"})

	path, err := wailsruntime.OpenFileDialog(s.ctx, wailsruntime.OpenDialogOptions{
		Title:   title,
		Filters: filters,
	})
	if err != nil {
		return "", wrapError(err)
	}
	return path, nil
}

// SaveTextFile raises the host's own save dialog and writes text to the path it was given, answering
// with that path — or with an empty one for a dialog the reader dismissed, exactly as ChooseFile
// reads a dismissal.
//
// It is the picker's twin and lives here for the same reason: what is written is the *frontend's*
// own — a diagram as SVG — rather than anything a server answered with, so there is no session to
// hold and no driver in it. Everything a server produces goes out through ExportRows or DumpObjects,
// which stream what they read rather than being handed it.
func (s *SystemAPI) SaveTextFile(title, pattern, name, content string) (string, error) {
	filters := []wailsruntime.FileFilter{}
	if pattern != "" {
		filters = append(filters, wailsruntime.FileFilter{DisplayName: title, Pattern: pattern})
	}
	filters = append(filters, wailsruntime.FileFilter{DisplayName: "All files", Pattern: "*.*"})

	path, err := wailsruntime.SaveFileDialog(s.ctx, wailsruntime.SaveDialogOptions{
		Title:           title,
		DefaultFilename: name,
		Filters:         filters,
	})
	if err != nil {
		return "", wrapError(err)
	}
	if path == "" {
		return "", nil
	}

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", wrapError(err)
	}
	return path, nil
}
