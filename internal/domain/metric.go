package domain

// MetricPanel is which of four questions about a live server a number answers. It is a closed list
// for the reason ProcessAction is one: the frontend draws a panel per entry, and a word that arrived
// as something else would be a chart with nothing in it.
//
// The **panels** are fixed and the **series inside them** are the engine's own, which is the same
// arrangement a process list has — one shape, whatever columns that catalogue keeps. No two of these
// engines count the same things: MySQL counts bytes on the wire and PostgreSQL counts blocks off the
// disk, Oracle counts logical reads and Redis counts key-space hits. A vocabulary that insisted they
// were all the same number would be this app inventing a fact, so what a driver hands over is its own
// name for its own counter, sorted into the question it helps answer.
type MetricPanel string

const (
	// MetricSessions is who is connected: how many, and how many of those are doing something. Every
	// series here is a gauge — a number that is true now rather than one that has been adding up.
	MetricSessions MetricPanel = "sessions"
	// MetricTraffic is how much data is moving, in bytes. Two engines count it on the wire and the
	// rest count it off the disk, and the series says which — a label is where that is answered,
	// never a panel that pretends the two are one number.
	MetricTraffic MetricPanel = "traffic"
	// MetricQueries is how much work is being asked for: statements, commands, commits, rollbacks.
	MetricQueries MetricPanel = "queries"
	// MetricThroughput is what that work moved — rows, documents, blocks, key-space hits. It is the
	// panel that says whether a quiet-looking server is quiet, a hundred statements an hour that each
	// read a million rows being the busiest server there is.
	MetricThroughput MetricPanel = "throughput"
)

// MetricKind is how a number is read, and it is the one fact about a metric the frontend cannot
// guess. A gauge is drawn as it stands; a counter has only ever gone up since the server started, so
// what is drawn is the **difference between two samples over the time between them** — a rate.
//
// It is the driver's to say because only the engine knows which of its own numbers is which:
// `Threads_connected` and `Bytes_received` sit in the same list and mean opposite things.
type MetricKind string

const (
	// MetricGauge is true at the moment it was read and means nothing subtracted from the last one.
	MetricGauge MetricKind = "gauge"
	// MetricCounter has been adding up since the server started. A single sample of one says almost
	// nothing, which is why the chart it lands in is empty until the second sample arrives.
	MetricCounter MetricKind = "counter"
)

// MetricUnit is what the number is counted in, and it decides how it is written out: a rate of bytes
// reads as `1.4 MB/s` and a rate of anything else as `320/s`. Two units are enough because there are
// only two kinds of thing any of these engines counts.
type MetricUnit string

const (
	// MetricCount is a plain tally — sessions, statements, rows, documents, keys.
	MetricCount MetricUnit = "count"
	// MetricBytes is a size, and is the reason a unit exists at all: nothing else here needs to be
	// divided by 1024 before a reader can read it.
	MetricBytes MetricUnit = "bytes"
)
