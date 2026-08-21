import type { MetricPanel, MetricUnit, ServerMetric, ServerMetrics } from '@/bridge/monitor'

/**
 * What the charts pane keeps and what it makes of it. Nothing here draws anything and nothing here
 * asks a server anything: it is the arithmetic between a list of numbers the driver handed over and
 * the four pictures `MetricsPane` draws — `arrange`'s counterpart for a time series.
 *
 * See [docs/features/metrics.md](../../../../docs/features/metrics.md).
 */

/**
 * How many readings are kept. At the fastest clock the pane offers that is eight minutes of history
 * and at the slowest two hours, which is the window a reader watching a server is asking about —
 * anything longer is what a monitoring system is for, and this app is not one.
 *
 * It is a ceiling on **memory** as much as on time: 240 readings of twenty numbers is nothing, and an
 * unbounded array behind a pane left open overnight would not be.
 */
export const HISTORY = 240

/**
 * How far back the readings may reach, whatever the count. It is the same two hours the slowest clock
 * would fill `HISTORY` with, and it is a **second** ceiling because a count alone stopped bounding the
 * time once the readings began outliving the pane: a tab left open for a week and glanced at each
 * morning would hold seven readings a week apart, and — the axis being time — squash all of today
 * into the last pixel of it. A number about Monday is not what this pane is for.
 */
export const WINDOW = 2 * 60 * 60 * 1000

/**
 * The four panels, in the order they are drawn, and what each is asking. The hint is drawn under the
 * title because a panel's honest answer differs per engine — a traffic chart is bytes on the wire on
 * four of these engines and blocks off the disk on two — and the **series labels** are where that is
 * said, so the hint says what the panel is *for* rather than what it holds.
 */
export const PANELS: ReadonlyArray<{ panel: MetricPanel; title: string; hint: string }> = [
  {
    panel: 'sessions',
    title: 'Sessions',
    hint: 'Who is connected, and how many of them are doing something',
  },
  { panel: 'traffic', title: 'Traffic', hint: 'How much data the server is moving' },
  { panel: 'queries', title: 'Queries', hint: 'How much work is being asked for' },
  { panel: 'throughput', title: 'Throughput', hint: 'What that work actually moved' },
]

/** One reading of every number, kept exactly as it arrived. */
export type Reading = ServerMetrics

/**
 * Adds a reading to the history, oldest first, keeping the last `HISTORY` of them and none older
 * than `WINDOW`.
 *
 * A reading whose clock is **not after** the one before it is dropped rather than appended: a rate is
 * a difference divided by the time between two samples, so a duplicate or an out-of-order answer
 * would divide by nothing. `useServerReport` already drops a late answer to a *different* question;
 * this is the same care taken against one server answering the same question twice in a millisecond.
 */
export function remember(history: readonly Reading[], reading: Reading): Reading[] {
  const last = history[history.length - 1]
  if (last !== undefined && reading.readAt <= last.readAt) return history as Reading[]

  const oldest = reading.readAt - WINDOW
  return [...history, reading].slice(-HISTORY).filter((held) => held.readAt >= oldest)
}

/** One line on a chart: what it is called, and what it was worth at each of the shared instants. */
export interface Series {
  key: string
  label: string
  unit: MetricUnit
  /** One entry per instant, `null` where there is no number for it — see `rates`. */
  values: (number | null)[]
}

/** One panel's worth of lines over one shared clock, which is what a chart is handed. */
export interface Chart {
  panel: MetricPanel
  title: string
  hint: string
  /** The instant each point belongs to, in milliseconds. Shared by every series on the panel. */
  times: number[]
  series: Series[]
  /** Whether these are rates rather than levels, which is the whole of how they are written out. */
  rate: boolean
  /** What the axis is counted in. A panel's series are one unit; the first of them is the panel's. */
  unit: MetricUnit
}

/**
 * Turns the readings kept so far into the four charts.
 *
 * **Which lines exist is the latest reading's answer**, never the union of every reading: a counter
 * the server stopped reporting is one it has not got any more, and a line held on the panel by a
 * memory of it would be a line that never moves again. A line that is *new* simply starts partway
 * along, which is what the nulls in front of it draw.
 *
 * **A counter becomes a rate here**, because a number that has been adding up since a server started
 * is not something a chart can show: what is drawn is the difference between two readings over the
 * seconds between them. That is why a counter panel is empty until the second reading arrives, and
 * why the first point of every one of them is `null` rather than zero.
 */
export function charts(history: readonly Reading[]): Chart[] {
  const latest = history[history.length - 1]
  const times = history.map((reading) => reading.readAt)

  return PANELS.map(({ panel, title, hint }) => {
    const held = (latest?.metrics ?? []).filter((metric) => metric.panel === panel)
    return {
      panel,
      title,
      hint,
      times,
      series: held.map((metric) => line(history, metric)),
      rate: held[0]?.kind === 'counter',
      unit: held[0]?.unit ?? 'count',
    }
  })
}

/**
 * One metric's line across the whole history. A reading that did not hold this key contributes
 * `null`, which is a gap in the line rather than a zero on it: a server that stopped answering for
 * one number has not started reporting none of it.
 */
function line(history: readonly Reading[], metric: ServerMetric): Series {
  const raw = history.map((reading) => valueOf(reading, metric.key))
  const values = metric.kind === 'counter' ? rates(raw, history) : raw
  return { key: metric.key, label: metric.label, unit: metric.unit, values }
}

function valueOf(reading: Reading, key: string): number | null {
  const metric = reading.metrics.find((held) => held.key === key)
  return metric === undefined ? null : metric.value
}

/**
 * A counter's readings turned into what it was doing per second between them. The first point is
 * always `null` — one reading of a counter says only what the server has done since it started, which
 * is a number about last week rather than about now.
 *
 * **A reading lower than the one before it is dropped**, not drawn as a fall: a counter only ever goes
 * up, so a smaller number means the server restarted or was told to forget — Redis's `CONFIG
 * RESETSTAT`, a MySQL `FLUSH STATUS` — and drawing that as a plunge would say the server did
 * something it did not do.
 */
function rates(raw: (number | null)[], history: readonly Reading[]): (number | null)[] {
  return raw.map((value, at) => {
    const before = raw[at - 1]
    if (at === 0 || value === null || before === null || before === undefined) return null

    const now = history[at]?.readAt
    const then = history[at - 1]?.readAt
    if (now === undefined || then === undefined) return null

    const seconds = (now - then) / 1000
    if (seconds <= 0 || value < before) return null
    return (value - before) / seconds
  })
}

/**
 * What one number reads as, in the unit it is counted in and per second where it is a rate. It is the
 * one place a number on this pane is written out, so an axis label, a legend and a tooltip cannot
 * round the same value two different ways.
 */
export function say(value: number | null, unit: MetricUnit, rate: boolean): string {
  if (value === null) return '—'
  const written = unit === 'bytes' ? bytes(value) : count(value)
  return rate ? `${written}/s` : written
}

const BYTE_STEPS = ['B', 'KB', 'MB', 'GB', 'TB']
const COUNT_STEPS = ['', 'k', 'M', 'G']

function bytes(value: number): string {
  let step = 0
  let held = value
  while (held >= 1024 && step < BYTE_STEPS.length - 1) {
    held /= 1024
    step += 1
  }
  return `${figures(held, step === 0)} ${BYTE_STEPS[step]}`
}

function count(value: number): string {
  let step = 0
  let held = value
  while (held >= 1000 && step < COUNT_STEPS.length - 1) {
    held /= 1000
    step += 1
  }
  return `${figures(held, step === 0)}${COUNT_STEPS[step]}`
}

/**
 * How many figures a scaled number is written to. A number that was never divided keeps none of its
 * own — `4 clients` rather than `4.0` — and anything that was keeps one, which is what makes a column
 * of them the same width while a server is busy.
 */
function figures(value: number, whole: boolean): string {
  if (whole) return `${Math.round(value * 10) / 10}`
  return value.toFixed(1)
}

/**
 * The top of a chart's axis: the largest value on it, rounded **up** to something a reader can divide
 * by eye — 1, 2 or 5 times a power of ten. An axis that ended at the exact maximum would move on
 * every reading, and a chart whose gridlines shuffle while it is being watched is one nobody can read
 * a level off.
 *
 * A panel holding nothing but zeroes gets an axis of 1, so the line is drawn along the bottom rather
 * than divided by nothing.
 */
export function ceiling(series: readonly Series[]): number {
  let most = 0
  for (const held of series) {
    for (const value of held.values) {
      if (value !== null && value > most) most = value
    }
  }
  if (most <= 0) return 1

  const power = 10 ** Math.floor(Math.log10(most))
  for (const step of [1, 2, 5, 10]) {
    if (most <= step * power) return step * power
  }
  return 10 * power
}

/**
 * The clock time one instant reads as, which is what the two ends of the axis are labelled with. An
 * instant that is not there at all reads as nothing rather than as the epoch: a chart with no
 * readings yet has both ends missing, and `1970` under one would be a time the server never saw.
 */
export function clock(at: number | undefined): string {
  return at === undefined ? '' : new Date(at).toLocaleTimeString(undefined, { hour12: false })
}

/**
 * Where one reading sits along the axis, as a fraction of the window: its own instant between the
 * first reading's and the last's.
 *
 * **It is the time and not the place in the run**, because the clock changes while readings
 * accumulate. Twenty readings taken 30s apart and twenty taken 2s apart are the same *count* and
 * forty times the stretch, so spacing them evenly would draw ten minutes and forty seconds the same
 * width — and the two labels under the chart, which are the readings' own clock times, would be
 * describing a picture that did not honour them. The same goes for a stretch with `Auto: Off` in the
 * middle of it, and for a server that answered slower than it was asked.
 *
 * A window with one reading in it, or with no width, answers `1`: the single point sits at the
 * present rather than at a beginning it is also the end of.
 */
export function spanAt(times: readonly number[], at: number): number {
  const first = times[0]
  const last = times[times.length - 1]
  const held = times[at]
  if (first === undefined || last === undefined || held === undefined) return 1

  const span = last - first
  return span <= 0 ? 1 : (held - first) / span
}

/**
 * Which reading an axis fraction is nearest, by **time** — `spanAt` read backwards, and what turns a
 * pointer somewhere along a panel into the one instant the whole panel reads itself out at. Nearest
 * by time rather than by place for `spanAt`'s reason: after the clock has been changed, a pointer
 * halfway across is not over the halfway reading.
 */
export function nearest(times: readonly number[], fraction: number): number {
  const first = times[0]
  const last = times[times.length - 1]
  if (first === undefined || last === undefined) return 0

  const wanted = first + (last - first) * Math.min(1, Math.max(0, fraction))
  let best = 0
  let closest = Infinity
  times.forEach((time, at) => {
    const away = Math.abs(time - wanted)
    if (away < closest) {
      closest = away
      best = at
    }
  })
  return best
}
