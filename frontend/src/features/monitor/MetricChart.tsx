import { useState } from 'react'

import { ceiling, clock, nearest, say, spanAt, type Chart, type Series } from './metrics'

/**
 * One of the four charts: a panel's lines over the readings kept so far, the legend that says what
 * each one is worth now, and the crosshair a pointer reads a moment off.
 *
 * **It is drawn as an SVG stretched to whatever the panel is**, which is what
 * `preserveAspectRatio="none"` does, and every stroke on it carries `vector-effect` so a wide panel
 * does not draw a thick line and a short one a thin one. Nothing inside the SVG is text for the same
 * reason — a label would be stretched with it — and nothing in it is round, an ellipse being what a
 * circle becomes: the axis, the legend and the dots are ordinary elements over the top of it.
 *
 * **A point sits at its own instant**, not at its place in the run: see `spanAt` for what goes wrong
 * otherwise, the clock being a thing the reader changes while readings accumulate.
 *
 * **The crosshair belongs to the pane and not to this panel.** The four panels are drawn against one
 * clock so that they can be read against each other — a traffic chart that climbed while the session
 * count did not is a different answer from one where both did — and reading two panels against each
 * other means reading them at the same moment. So `at` arrives as a prop.
 *
 * **There is no tooltip, because the legend is one.** A panel this size has its numbers written out
 * three lines below the pointer already, so a box that followed the cursor would be the same figures
 * twice, over the lines they are about.
 *
 * See [docs/features/metrics.md](../../../../docs/features/metrics.md).
 */

/** The height the paths are drawn against. Any number does; the SVG is stretched to the panel. */
const PLOT = 300

/** The width likewise, so a point's x is a fraction of the window the readings cover. */
const SPAN = 1000

/**
 * How many colours a panel cycles through. Six is as many series as any of these engines puts on one
 * panel — MongoDB puts exactly six on two of them — and a table that outran it would draw two lines
 * alike, which is why `driver.MetricColours` says the same number on the Go side and every adapter's
 * own test checks its table against it.
 */
const COLOURS = 6

/**
 * The patterns a line is drawn with, cycled by its place on the panel, on and off in the plot's own
 * units.
 *
 * It is a **second** channel beside the colour, and it is here because colour alone is one channel:
 * six hues are six distinguishable lines for most readers and fewer for a reader who cannot separate
 * the green from the amber, which on these panels are neighbours. Three patterns over six colours
 * means no two adjacent series share either.
 */
const PATTERNS: readonly (readonly [number, number] | null)[] = [null, [6, 4], [2, 3]]

export function MetricChart({
  chart,
  at,
  onAt,
}: {
  chart: Chart
  /**
   * Which reading the pane is reading out, and `null` for a pointer that is on none of the panels. It
   * is an **index** into the shared clock rather than a position, so the crosshair, every dot, the
   * time under it and every number in all four legends are one instant.
   */
  at: number | null
  onAt: (at: number | null) => void
}) {
  /**
   * Which single series the panel has been cut down to, and `null` for all of them.
   *
   * It is here because one axis over series of different magnitudes is an axis that hides most of
   * them: `Innodb_rows_read` runs four orders of magnitude above `Innodb_rows_updated`, and against a
   * ceiling taken from the first the second is drawn along the floor, where a real trickle and a
   * dead stop look the same. Isolating one re-takes the ceiling from it alone.
   */
  const [only, setOnly] = useState<string | null>(null)

  const points = chart.times.length

  // A series the server has stopped reporting cannot be the one the panel is cut down to: the panel
  // would draw nothing while its legend showed lines. It un-isolates itself rather than emptying.
  const isolated = only !== null && chart.series.some((series) => series.key === only) ? only : null

  // The lines to draw, each carrying **its own place on the panel** so that cutting the panel down
  // does not recolour what is left: a series is one colour and one pattern for as long as it is
  // reported, whatever else is drawn beside it.
  const drawn = chart.series
    .map((series, index) => ({ series, index }))
    .filter(({ series }) => isolated === null || series.key === isolated)

  const top = ceiling(drawn.map((held) => held.series))

  /** Which reading the panel is reading out: whatever the pointer is on, or the latest there is. */
  const reading = at === null ? points - 1 : at

  return (
    <div className="flex min-h-0 min-w-0 flex-col rounded-md border border-line bg-panel p-2.5">
      <div className="flex shrink-0 items-baseline justify-between gap-2">
        <span className="text-xs font-medium text-fg">{chart.title}</span>
        <span className="truncate text-2xs text-faint">{chart.hint}</span>
      </div>

      {chart.series.length === 0 ? (
        <div className="flex min-h-0 flex-1 items-center justify-center px-3 text-center text-2xs text-faint">
          This server counts nothing here.
        </div>
      ) : (
        <>
          <div
            className="relative mt-2 min-h-0 flex-1"
            onPointerMove={(event) => {
              const box = event.currentTarget.getBoundingClientRect()
              if (points < 2 || box.width === 0) return
              // Nearest by **time**, which is what the pointer is over: after the clock has been
              // changed, halfway across the panel is not the halfway reading.
              onAt(nearest(chart.times, (event.clientX - box.left) / box.width))
            }}
            onPointerLeave={() => onAt(null)}
          >
            <svg
              className="absolute inset-0 h-full w-full"
              viewBox={`0 0 ${SPAN} ${PLOT}`}
              preserveAspectRatio="none"
              aria-hidden
            >
              {[0, 0.25, 0.5, 0.75, 1].map((fraction) => (
                <line
                  key={fraction}
                  x1={0}
                  x2={SPAN}
                  y1={PLOT * fraction}
                  y2={PLOT * fraction}
                  stroke="var(--chart-grid)"
                  strokeWidth={1}
                  vectorEffect="non-scaling-stroke"
                />
              ))}

              {drawn.map(({ series, index }) => (
                <g key={series.key}>
                  <path
                    d={area(series, chart.times, top)}
                    fill={colour(index)}
                    fillOpacity={0.1}
                    stroke="none"
                  />
                  <path
                    d={stroke(series, chart.times, top)}
                    fill="none"
                    stroke={colour(index)}
                    strokeWidth={1.5}
                    strokeDasharray={dashes(index)}
                    strokeLinejoin="round"
                    strokeLinecap="round"
                    vectorEffect="non-scaling-stroke"
                  />
                </g>
              ))}

              {at !== null && points > 1 ? (
                <line
                  x1={along(chart.times, at)}
                  x2={along(chart.times, at)}
                  y1={0}
                  y2={PLOT}
                  stroke="var(--chart-crosshair)"
                  strokeWidth={1}
                  vectorEffect="non-scaling-stroke"
                />
              ) : null}
            </svg>

            {/* The axis. Three of the five gridlines are labelled — the top, the middle and the
                floor — because a panel this size holds five numbers up its left edge and reads as a
                wall of them. They sit **over** the plot rather than beside it so every panel's lines
                start at the same x however wide its numbers are. */}
            {[1, 0.5, 0].map((fraction) => (
              <span
                key={fraction}
                className="pointer-events-none absolute left-0 -translate-y-1/2 bg-panel/80 pr-1 text-2xs tabular-nums text-faint"
                style={{ top: `${(1 - fraction) * 100}%` }}
              >
                {say(top * fraction, chart.unit, chart.rate)}
              </span>
            ))}

            {/* The dot each line is carrying at the moment under the pointer. They are elements
                rather than circles in the SVG because a circle in a stretched drawing is an
                ellipse — the same reason no label is in there. */}
            {at !== null && points > 1
              ? drawn.map(({ series, index }) => {
                  const value = series.values[at]
                  if (value === null || value === undefined) return null
                  return (
                    <span
                      key={series.key}
                      className="pointer-events-none absolute size-1.5 -translate-x-1/2 -translate-y-1/2 rounded-full"
                      style={{
                        left: `${spanAt(chart.times, at) * 100}%`,
                        top: `${(1 - Math.min(1, value / top)) * 100}%`,
                        background: colour(index),
                      }}
                    />
                  )
                })
              : null}
          </div>

          {/* The two ends of the clock, which is what says how far back the panel goes. It is the
              reading's own time under the pointer rather than the latest one, so the crosshair says
              *when* as well as how much. */}
          <div className="mt-1 flex shrink-0 justify-between text-2xs tabular-nums text-faint">
            <span>{clock(chart.times[0])}</span>
            <span>{clock(chart.times[reading])}</span>
          </div>

          {/* What each line is worth at the moment being read — now, or wherever the pointer is. It
              is a wrapping row of swatches rather than a grid: a legend is a key to a picture, and
              anything in this app that is rows and columns is `ResultGrid`.

              Each entry is also the control that cuts the panel down to that one line, which is the
              only thing on these four panels there is to hide — and the thing an axis shared between
              magnitudes makes necessary. Every line stays in the legend while one is isolated, dimmed
              rather than dropped, so the way back is where the way in was. */}
          <div className="mt-1.5 flex shrink-0 flex-wrap gap-x-3 gap-y-1">
            {chart.series.map((series, index) => {
              const hidden = isolated !== null && isolated !== series.key
              return (
                <button
                  key={series.key}
                  type="button"
                  aria-pressed={isolated === series.key}
                  title={
                    isolated === series.key
                      ? `Show every line on ${chart.title} again`
                      : `Show only ${series.label}, and take the axis from it`
                  }
                  onClick={() => setOnly(isolated === series.key ? null : series.key)}
                  // The padding is cancelled by the margin, so the pressable area is wider than the
                  // words without the legend's own spacing moving to make room for it.
                  className={`-mx-1 flex items-center gap-1.5 rounded px-1 text-2xs outline-none hover:bg-hover focus-visible:ring-1 focus-visible:ring-accent ${
                    hidden ? 'opacity-40' : ''
                  }`}
                >
                  <span
                    className="h-0.5 w-3 shrink-0"
                    style={{ background: swatch(index) }}
                  />
                  <span className="text-muted">{series.label}</span>
                  <span className="tabular-nums text-fg">
                    {say(series.values[reading] ?? null, series.unit, chart.rate)}
                  </span>
                </button>
              )
            })}
          </div>
        </>
      )}
    </div>
  )
}

/** Which of the panel's colours a line is drawn in, by its place on the panel. */
function colour(index: number): string {
  return `var(--chart-${(index % COLOURS) + 1})`
}

/** The on-and-off a line is drawn with, or nothing at all where it is a solid one. */
function pattern(index: number): readonly [number, number] | null {
  return PATTERNS[index % PATTERNS.length] ?? null
}

function dashes(index: number): string | undefined {
  const held = pattern(index)
  return held === null ? undefined : `${held[0]} ${held[1]}`
}

/**
 * The legend's mark for one line: the same colour, and the same pattern said in CSS because the mark
 * is an ordinary element rather than something in the stretched SVG. A key that showed only the
 * colour would be a key to half of what distinguishes the lines.
 */
function swatch(index: number): string {
  const held = pattern(index)
  const paint = colour(index)
  if (held === null) return paint

  const [on, off] = held
  return `repeating-linear-gradient(90deg, ${paint} 0 ${on}px, transparent ${on}px ${on + off}px)`
}

/** Where one reading sits across the plot, by its own instant — see `spanAt`. */
function along(times: readonly number[], at: number): number {
  return spanAt(times, at) * SPAN
}

/** How high one value sits, with the axis's top as the ceiling. */
function height(value: number, top: number): number {
  return PLOT - Math.min(1, value / top) * PLOT
}

/**
 * The line itself, drawn as one path with a **break at every gap**. A null is a reading this number
 * was not in, or the first reading of a counter, and joining across one would draw a straight line
 * through minutes the server never reported.
 */
function stroke(series: Series, times: readonly number[], top: number): string {
  let path = ''
  let drawing = false

  series.values.forEach((value, at) => {
    if (value === null) {
      drawing = false
      return
    }
    const command = drawing ? 'L' : 'M'
    path += `${command}${along(times, at).toFixed(2)} ${height(value, top).toFixed(2)}`
    drawing = true
  })
  return path
}

/**
 * The wash under the line, which is one shape per unbroken run rather than one under the whole
 * series: a fill closed across a gap would colour in a stretch of time that has no line over it.
 */
function area(series: Series, times: readonly number[], top: number): string {
  let path = ''
  let run: number[] = []

  const close = () => {
    if (run.length < 2) {
      run = []
      return
    }
    const first = run[0] ?? 0
    const last = run[run.length - 1] ?? 0

    path += `M${along(times, first).toFixed(2)} ${PLOT}`
    for (const at of run) {
      path += `L${along(times, at).toFixed(2)} ${height(series.values[at] ?? 0, top).toFixed(2)}`
    }
    path += `L${along(times, last).toFixed(2)} ${PLOT}Z`
    run = []
  }

  series.values.forEach((value, at) => {
    if (value === null) {
      close()
      return
    }
    run.push(at)
  })
  close()
  return path
}
