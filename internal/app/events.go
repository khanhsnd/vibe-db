package app

// The two events of a streamed read. A read of every row of a large table cannot be answered in
// one piece — a bound method's result crosses the bridge as a single JavaScript string — so
// LoadTableRows answers such a request with its columns alone and pushes the rest.
//
// They are two events rather than one because they travel at different rates, which is the whole
// point: rows go out in batches of `sqlkit.BatchRows`, and those land on round counts, while the
// count goes out on `sqlkit.CountEvery` so a reader watching it sees the number the read has truly
// reached and sees it climb rather than step.
const (
	EventRows        = "rows:batch"
	EventRowsCounted = "rows:counted"
)

// RowsBatch is the rows a streamed read has just produced, in the order it produced them.
type RowsBatch struct {
	RunID string      `json:"runId"`
	Rows  [][]*string `json:"rows"`
}

// RowsCounted is how many rows a streamed read has produced so far — every row of it, not only
// the ones in the last batch.
type RowsCounted struct {
	RunID   string `json:"runId"`
	Fetched int    `json:"fetched"`
}
