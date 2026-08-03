package app

// EventRows carries the rows of a streamed read as they arrive. A read of every row of a large
// table cannot be answered in one piece — a bound method's result crosses the bridge as a single
// JavaScript string — so LoadTableRows answers such a request with its columns alone and emits
// the rows here, batch by batch, under the run id the caller minted.
const EventRows = "rows:batch"

// RowsBatch is one batch of a streamed read. Fetched counts every row the read has produced,
// this batch included, which is the number a reader watches while it runs.
type RowsBatch struct {
	RunID   string      `json:"runId"`
	Fetched int         `json:"fetched"`
	Rows    [][]*string `json:"rows"`
}
