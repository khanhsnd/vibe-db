package driver

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/khanhsnd/vibe-db/internal/domain"
)

// Metric is one number a live server keeps about itself, named the way that engine names it and
// sorted into the question it helps answer. It is ProcessList's arrangement applied to a chart: the
// **panels** are fixed and the **series** are the engine's own, because no two of these engines count
// the same things and a vocabulary insisting they did would be this app inventing a fact.
//
// Key is what the *server* calls it and is never drawn — it is what keeps one line the same line
// across two samples, so a metric that comes and goes does not become a second series. Label is what
// a reader sees, and is the adapter's to write for the reason a column heading is.
type Metric struct {
	Panel domain.MetricPanel
	Key   string
	Label string
	Kind  domain.MetricKind
	Unit  domain.MetricUnit
	Value float64
}

// ServerMetrics is one read of every number the four charts are drawn from, taken at one moment. It
// is one answer rather than four because a reader watching a chart is watching one server: two
// numbers that landed in different reads would be plotted at the same instant while describing two.
type ServerMetrics struct {
	Metrics []Metric
}

// MetricSpec names one number an adapter is looking for in what its engine already answers with: the
// engine's own name for it, what it is called on the chart, and the two facts nothing outside a
// driver could know — whether it has been adding up since the server started, and whether it is
// counted in bytes.
//
// It is a *table* per adapter rather than a run of appends because that is what these reads are: four
// engines answer with name-and-value pairs, and picking the fifteen that matter out of a thousand is
// one loop over one list.
type MetricSpec struct {
	Panel domain.MetricPanel
	Name  string
	Label string
	Kind  domain.MetricKind
	Unit  domain.MetricUnit
}

// MetricColours is how many colours a chart cycles through, and so is the ceiling on how many series
// one panel may hold. It is the frontend's own `COLOURS` said again here, because it is only here
// that a table can be checked against it: a seventh series on one panel would be handed the first
// one's colour, and two lines the same colour is a panel a reader cannot read.
const MetricColours = 6

// PanelFault says what is wrong with one adapter's table of specs, and "" where nothing is. It is
// the invariant the four charts are drawn on, written down beside the MetricSpec it is about:
//
//   - **One panel is one kind.** A gauge is drawn as it stands and a counter as a rate, and a panel
//     takes that answer from the first series it holds — so a gauge among counters is a level divided
//     by the seconds between two readings, which is a number about nothing.
//   - **One panel is one unit.** The axis is written out once for the whole panel, so a size among
//     tallies would have a megabyte labelled `50/s`.
//   - **One panel holds at most MetricColours series**, or two of its lines are drawn alike.
//
// None of the three is anything a compiler can see and all three fail *quietly*: what reaches the
// reader is a chart that looks perfectly ordinary. Each adapter's table is checked by that adapter's
// own test, the table being that adapter's to write.
func PanelFault(specs []MetricSpec) string {
	kinds := make(map[domain.MetricPanel]domain.MetricKind, 4)
	units := make(map[domain.MetricPanel]domain.MetricUnit, 4)
	held := make(map[domain.MetricPanel]int, 4)

	for _, spec := range specs {
		if kind, seen := kinds[spec.Panel]; seen && kind != spec.Kind {
			return fmt.Sprintf("panel %q holds a %s beside a %s: %q", spec.Panel, kind, spec.Kind, spec.Name)
		}
		if unit, seen := units[spec.Panel]; seen && unit != spec.Unit {
			return fmt.Sprintf("panel %q holds %s beside %s: %q", spec.Panel, unit, spec.Unit, spec.Name)
		}

		kinds[spec.Panel] = spec.Kind
		units[spec.Panel] = spec.Unit
		held[spec.Panel]++
		if held[spec.Panel] > MetricColours {
			return fmt.Sprintf("panel %q holds %d series and there are %d colours", spec.Panel, held[spec.Panel], MetricColours)
		}
	}
	return ""
}

// PickMetrics reads a table of specs against the pairs an engine answered with, and is how the four
// engines whose status *is* a list of names and values fill their charts.
//
// A name the server did not report, or reported as something that is not a number, contributes
// **nothing**: a counter this version does not keep is left out rather than drawn as a flat line at
// zero, those being opposite answers. Order follows the table, so a panel's series are drawn in the
// order the adapter listed them rather than in whatever order a map iterated.
func PickMetrics(specs []MetricSpec, pairs map[string]string) []Metric {
	metrics := make([]Metric, 0, len(specs))
	for _, spec := range specs {
		text, held := pairs[spec.Name]
		if !held {
			continue
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if err != nil {
			continue
		}
		metrics = append(metrics, spec.Metric(value))
	}
	return metrics
}

// Metric is one spec and the number that answered it. It is exported because the two engines whose
// numbers are not a list of pairs — PostgreSQL's sessions are counted and SQL Server's bytes are
// summed — build their own values and still say what they are through the same table.
func (spec MetricSpec) Metric(value float64) Metric {
	return Metric{
		Panel: spec.Panel,
		Key:   spec.Name,
		Label: spec.Label,
		Kind:  spec.Kind,
		Unit:  spec.Unit,
		Value: value,
	}
}

// MetricPairs reads a name-and-value answer out of a grid the adapter has already built — which is
// what MySQL's `SHOW GLOBAL STATUS`, Oracle's `v$sysstat` and a flattened `serverStatus` all are. It
// takes the columns by **name** rather than by place, for the reason `ProcessRefs` does.
//
// A name repeated wins on its first row, `v$sysstat` on a RAC instance being the case that matters:
// the rows are one instance's and the second is somebody else's.
func MetricPairs(columns []Column, rows [][]*string, name, value string) map[string]string {
	at := func(wanted string) int {
		for i, column := range columns {
			if strings.EqualFold(column.Name, wanted) {
				return i
			}
		}
		return -1
	}

	nameAt, valueAt := at(name), at(value)
	pairs := make(map[string]string, len(rows))
	if nameAt < 0 || valueAt < 0 {
		return pairs
	}

	for _, row := range rows {
		if nameAt >= len(row) || valueAt >= len(row) || row[nameAt] == nil || row[valueAt] == nil {
			continue
		}
		if _, held := pairs[*row[nameAt]]; held {
			continue
		}
		pairs[*row[nameAt]] = *row[valueAt]
	}
	return pairs
}

// SortMetrics puts a list into the panels' own order, leaving the series inside each panel where the
// adapter put them. It is what lets an adapter build its four tables in whatever order reads best and
// still hand back a list the panels are drawn from in one pass.
func SortMetrics(metrics []Metric) []Metric {
	order := map[domain.MetricPanel]int{
		domain.MetricSessions:   0,
		domain.MetricTraffic:    1,
		domain.MetricQueries:    2,
		domain.MetricThroughput: 3,
	}
	sort.SliceStable(metrics, func(a, b int) bool {
		return order[metrics[a].Panel] < order[metrics[b].Panel]
	})
	return metrics
}
