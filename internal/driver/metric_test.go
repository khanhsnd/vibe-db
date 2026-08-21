package driver_test

import (
	"fmt"
	"testing"

	"github.com/khanhsnd/vibe-db/internal/domain"
	"github.com/khanhsnd/vibe-db/internal/driver"
)

// The half of a server's metrics that does not vary by engine: picking the numbers a chart is drawn
// from out of whatever that engine already answers with. What a *particular* engine reports is only
// answerable by a real server — see the `metrics` subtest of each adapter's `TestLiveMonitor` — so
// what is here is the machinery those four reads all go through.

func text(value string) *string { return &value }

var pairColumns = []driver.Column{
	{Name: "Variable_name", Type: "string"},
	{Name: "Value", Type: "string"},
}

func TestPickMetricsLeavesOutWhatTheServerDidNotSay(t *testing.T) {
	specs := []driver.MetricSpec{
		{Panel: domain.MetricSessions, Name: "Threads_connected", Label: "Connected",
			Kind: domain.MetricGauge, Unit: domain.MetricCount},
		{Panel: domain.MetricTraffic, Name: "Bytes_received", Label: "In",
			Kind: domain.MetricCounter, Unit: domain.MetricBytes},
		// A counter this server's version has not got. A flat line at zero and a line the server
		// will not draw are opposite answers, so nothing is contributed for it at all.
		{Panel: domain.MetricTraffic, Name: "Bytes_teleported", Label: "Out",
			Kind: domain.MetricCounter, Unit: domain.MetricBytes},
		// A name the server answered with something that is not a number — MySQL's status holds
		// `Rsa_public_key` and `Ssl_cipher_list` beside its counters.
		{Panel: domain.MetricQueries, Name: "Ssl_cipher", Label: "Cipher",
			Kind: domain.MetricGauge, Unit: domain.MetricCount},
	}

	pairs := map[string]string{
		"Threads_connected": "12",
		"Bytes_received":    " 4096 ",
		"Ssl_cipher":        "TLS_AES_256_GCM_SHA384",
	}

	metrics := driver.PickMetrics(specs, pairs)
	if len(metrics) != 2 {
		t.Fatalf("picked %d metrics, wanted the 2 the server answered with: %+v", len(metrics), metrics)
	}

	if metrics[0].Key != "Threads_connected" || metrics[0].Value != 12 {
		t.Errorf("first = %+v", metrics[0])
	}
	if metrics[0].Label != "Connected" || metrics[0].Panel != domain.MetricSessions {
		t.Errorf("first lost its spec: %+v", metrics[0])
	}
	// The order is the table's, so a panel's series are drawn where the adapter listed them rather
	// than wherever a map happened to iterate.
	if metrics[1].Key != "Bytes_received" || metrics[1].Value != 4096 {
		t.Errorf("second = %+v", metrics[1])
	}
	if metrics[1].Kind != domain.MetricCounter || metrics[1].Unit != domain.MetricBytes {
		t.Errorf("second lost its spec: %+v", metrics[1])
	}
}

func TestMetricPairsReadsColumnsByName(t *testing.T) {
	// The value column is in front of the name column, which is what would break a read that took
	// them by place.
	columns := []driver.Column{{Name: "Value"}, {Name: "Variable_name"}}
	rows := [][]*string{
		{text("7"), text("Threads_running")},
		{text("9"), text("Threads_connected")},
	}

	pairs := driver.MetricPairs(columns, rows, "Variable_name", "Value")
	if pairs["Threads_running"] != "7" || pairs["Threads_connected"] != "9" {
		t.Fatalf("read the wrong columns: %+v", pairs)
	}
}

func TestMetricPairsKeepsTheFirstOfARepeatedName(t *testing.T) {
	// `v$sysstat` on a clustered instance answers with one set of rows per instance, and the second
	// set is somebody else's server.
	rows := [][]*string{
		{text("execute count"), text("100")},
		{text("execute count"), text("999")},
	}

	pairs := driver.MetricPairs(
		[]driver.Column{{Name: "Variable_name"}, {Name: "Value"}}, rows, "Variable_name", "Value")
	if pairs["execute count"] != "100" {
		t.Fatalf("kept %q, wanted the first row's 100", pairs["execute count"])
	}
}

func TestMetricPairsAnswersWithNothingForColumnsThatAreNotThere(t *testing.T) {
	rows := [][]*string{{text("Threads_connected"), text("3")}}

	pairs := driver.MetricPairs(pairColumns, rows, "Name", "Value")
	if len(pairs) != 0 {
		t.Fatalf("read %d pairs off columns it could not find: %+v", len(pairs), pairs)
	}
}

func TestMetricPairsSkipsARowWithNothingInIt(t *testing.T) {
	rows := [][]*string{
		{nil, text("3")},
		{text("Threads_connected"), nil},
		{text("Threads_running")},
		{text("Uptime"), text("60")},
	}

	pairs := driver.MetricPairs(pairColumns, rows, "Variable_name", "Value")
	if len(pairs) != 1 || pairs["Uptime"] != "60" {
		t.Fatalf("read %+v, wanted only the one complete row", pairs)
	}
}

func TestSortMetricsPutsThePanelsInOrderAndLeavesTheSeriesAlone(t *testing.T) {
	metrics := []driver.Metric{
		{Panel: domain.MetricThroughput, Key: "rows"},
		{Panel: domain.MetricSessions, Key: "connected"},
		{Panel: domain.MetricQueries, Key: "questions"},
		{Panel: domain.MetricSessions, Key: "running"},
		{Panel: domain.MetricTraffic, Key: "in"},
	}

	wanted := []string{"connected", "running", "in", "questions", "rows"}
	for at, metric := range driver.SortMetrics(metrics) {
		if metric.Key != wanted[at] {
			t.Fatalf("sorted to %v, wanted %v", keysOf(driver.SortMetrics(metrics)), wanted)
		}
	}
}

func keysOf(metrics []driver.Metric) []string {
	keys := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		keys = append(keys, metric.Key)
	}
	return keys
}

// PanelFault is the one thing here that is checked against a table rather than against a server: the
// three ways a panel can be built wrong all fail *quietly*, so what is proved below is that the
// checker each adapter's own test leans on actually catches them.

func TestPanelFaultPassesATableWithOneKindAndOneUnitPerPanel(t *testing.T) {
	specs := []driver.MetricSpec{
		{Panel: domain.MetricSessions, Name: "connected", Kind: domain.MetricGauge, Unit: domain.MetricCount},
		{Panel: domain.MetricSessions, Name: "running", Kind: domain.MetricGauge, Unit: domain.MetricCount},
		{Panel: domain.MetricTraffic, Name: "in", Kind: domain.MetricCounter, Unit: domain.MetricBytes},
		{Panel: domain.MetricTraffic, Name: "out", Kind: domain.MetricCounter, Unit: domain.MetricBytes},
	}
	if fault := driver.PanelFault(specs); fault != "" {
		t.Fatalf("faulted a sound table: %s", fault)
	}
}

// A gauge among counters is the one that matters most: the panel would divide a level by the seconds
// between two readings and draw the answer as a rate.
func TestPanelFaultCatchesTwoKindsOnOnePanel(t *testing.T) {
	specs := []driver.MetricSpec{
		{Panel: domain.MetricQueries, Name: "questions", Kind: domain.MetricCounter, Unit: domain.MetricCount},
		{Panel: domain.MetricQueries, Name: "in_flight", Kind: domain.MetricGauge, Unit: domain.MetricCount},
	}
	if driver.PanelFault(specs) == "" {
		t.Fatal("passed a panel holding a gauge beside a counter")
	}
}

func TestPanelFaultCatchesTwoUnitsOnOnePanel(t *testing.T) {
	specs := []driver.MetricSpec{
		{Panel: domain.MetricThroughput, Name: "rows", Kind: domain.MetricCounter, Unit: domain.MetricCount},
		{Panel: domain.MetricThroughput, Name: "bytes", Kind: domain.MetricCounter, Unit: domain.MetricBytes},
	}
	if driver.PanelFault(specs) == "" {
		t.Fatal("passed a panel holding a size beside a tally")
	}
}

// One more series than there are colours, which is a panel drawing two lines the same.
func TestPanelFaultCatchesMoreSeriesThanColours(t *testing.T) {
	specs := make([]driver.MetricSpec, 0, driver.MetricColours+1)
	for at := 0; at <= driver.MetricColours; at++ {
		specs = append(specs, driver.MetricSpec{
			Panel: domain.MetricQueries,
			Name:  fmt.Sprintf("com_%d", at),
			Kind:  domain.MetricCounter,
			Unit:  domain.MetricCount,
		})
	}
	if driver.PanelFault(specs) == "" {
		t.Fatalf("passed a panel holding %d series", len(specs))
	}
}

// The same count is sound when it is spread over the panels, a ceiling being per panel and not per
// table: MongoDB puts six on one and six on another.
func TestPanelFaultCountsSeriesPerPanelAndNotPerTable(t *testing.T) {
	specs := make([]driver.MetricSpec, 0, driver.MetricColours*2)
	for _, panel := range []domain.MetricPanel{domain.MetricQueries, domain.MetricThroughput} {
		for at := 0; at < driver.MetricColours; at++ {
			specs = append(specs, driver.MetricSpec{
				Panel: panel,
				Name:  fmt.Sprintf("%s_%d", panel, at),
				Kind:  domain.MetricCounter,
				Unit:  domain.MetricCount,
			})
		}
	}
	if fault := driver.PanelFault(specs); fault != "" {
		t.Fatalf("faulted six series on each of two panels: %s", fault)
	}
}
