/**
 * How many rows may cross the bridge. Mirrors the const block in `internal/app/dto.go`.
 */

export const QUERY_ROW_LIMITS = [100, 1000, 10_000] as const
export const DEFAULT_QUERY_ROWS = 1000

export const PAGE_SIZES = [100, 500, 1000] as const
export const DEFAULT_PAGE_SIZE = 100

/**
 * Ask for every row rather than a page or a capped read. It is a sentinel rather than 0 so a
 * missing count reaching Go keeps falling back to a bounded default — see `app.AllRows`.
 */
export const ALL_ROWS = -1
