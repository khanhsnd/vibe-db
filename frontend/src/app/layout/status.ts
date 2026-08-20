import type { ConnectionStatus } from '@/features/connections/tree'

/**
 * What the tab in front is pointed at, worked out once by the shell and drawn by two surfaces: the
 * banner at the top of the window, which carries *which connection this is*, and the bar along the
 * bottom, which carries what the last read of it cost. One shape, so the two can never disagree
 * about the connection they are describing.
 */
export interface StatusInfo {
  status: ConnectionStatus
  /** What the reader called this connection, which is what they know it by. */
  name: string | null
  target: string | null
  /** The engine of the connection being reported on, drawn as its chip. */
  engine: string | null
  /** What that connection is for, drawn as its badge. Empty for one that names none. */
  environment: string
  serverVersion: string | null
  /**
   * Whether this connection refuses to send anything that writes. It is the session's own answer —
   * see `domain.Access` — and it is reported here because a reader who cannot see it finds out by
   * being refused, which is the wrong moment to learn it.
   */
  readOnly: boolean
  /** Where the tab in front is pointed — `shop.public`. Empty where it names no namespace. */
  namespace: string
  /** How many connections are open at all, which is what says this bar reports on one of several. */
  connections: number
  rowCount: number | null
  elapsedMs: number | null
  /**
   * The wording of the last failure nothing else was waiting to hear about: a refresh that could not
   * read, a dump that could not be written. A failure a *dialog* asked for is answered in that
   * dialog and never here — this is for the ones asked for from a menu, which has nowhere to say it.
   */
  error: string | null
}
