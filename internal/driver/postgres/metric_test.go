package postgres

import (
	"testing"

	"github.com/khanhsnd/vibe-db/internal/driver"
)

// A panel is drawn against **one** axis and written out in **one** unit, and it takes both from the
// first series it holds — so a gauge listed among counters, or a size among tallies, is a chart that
// reads wrong while looking perfectly ordinary. A seventh series is the same kind of quiet: it is
// handed the first one's colour. None of the three is visible to a compiler, and the table is this
// adapter's own, so it is checked here — see `driver.PanelFault`.
func TestPostgresMetricsKeepOnePanelToOneAxis(t *testing.T) {
	if fault := driver.PanelFault(postgresMetrics); fault != "" {
		t.Fatalf("postgresMetrics: %s", fault)
	}
}
