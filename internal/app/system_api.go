package app

import "github.com/khanhsnd/vibe-db/internal/fonts"

// SystemAPI answers what the host machine offers rather than what a server does — today the font
// families Settings picks from. It holds no state and needs no session.
type SystemAPI struct{}

func NewSystemAPI() *SystemAPI {
	return &SystemAPI{}
}

// ListFonts answers the font families installed on this machine, sorted. It returns no error on
// purpose: a machine whose font list cannot be read is not a failure a reader can act on, and the
// picker falls back to the families the stylesheet ships with.
func (s *SystemAPI) ListFonts() []string {
	return fonts.Families()
}
