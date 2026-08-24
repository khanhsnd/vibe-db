import { cn } from '@/lib/cn'

/** The drawing's own proportions: a cat one and a half times as wide as it is tall. */
const RATIO = 1.5

interface RunningCatProps {
  /** How tall the cat is drawn, in pixels. The width follows from it. */
  size?: number
  className?: string
}

/**
 * A small cat running on the spot, beside the words that say something is running.
 *
 * It is a **silhouette**, which is what makes it readable at sixteen pixels: a cat that size is a
 * body, two ears, a tail and four legs, and one hole for an eye — punched with `evenodd` so the
 * surface behind shows through, the same trick an engine's chip uses for the elephant's eyes.
 *
 * It takes `currentColor` and never a colour of its own. A black cat on a dark bar is an invisible
 * cat, so what it actually wears is the text tone beside it: near-black on a light theme, and light
 * on a dark one.
 *
 * The run is **two frames** (`.cat-run` in `index.css`) — legs gathered, legs splayed — and no
 * interpolation between them, because a leg tweened across two pixels is a grey smear rather than a
 * stride. It runs in place: what *moves* is [the bar it stands beside](./BusyBar.tsx), and a cat
 * travelling across the toolbar as well would be two things saying one thing.
 */
export function RunningCat({ size = 16, className }: RunningCatProps) {
  return (
    <svg
      aria-hidden
      viewBox="0 0 30 20"
      height={size}
      width={size * RATIO}
      fill="currentColor"
      className={cn('cat-run shrink-0', className)}
    >
      {/* The tail is stroked rather than filled, being one long curve, and is the one part swept
          rather than swapped: a tail is long enough for two pixels of movement to read. */}
      <path
        className="cat-tail"
        fill="none"
        stroke="currentColor"
        strokeWidth="2.4"
        strokeLinecap="round"
        d="M7.8 11 C2.8 11.4 1.6 6.6 5.2 4.2"
      />

      {/* Body, ears and head ride together, so the bob lifts the cat and not its legs. */}
      <g className="cat-bob">
        <ellipse cx="13.4" cy="11" rx="7" ry="4.5" />
        <path d="M17.9 4.6 L18.6 0.9 L21.4 3.8 Z M22.8 3.2 L25.2 1.1 L25.6 5.0 Z" />
        <path
          fillRule="evenodd"
          d="M21.5 2.6 A4.5 4.5 0 1 1 21.5 11.6 A4.5 4.5 0 1 1 21.5 2.6 Z
             M22.9 5.2 A0.9 0.9 0 1 1 22.9 7.0 A0.9 0.9 0 1 1 22.9 5.2 Z"
        />
      </g>

      <g className="cat-frame-a">
        <rect x="8.2" y="13.8" width="2.2" height="4.6" rx="1.1" />
        <rect x="11.4" y="13.8" width="2.2" height="4.6" rx="1.1" />
        <rect x="15.8" y="13.8" width="2.2" height="4.6" rx="1.1" />
        <rect x="18.6" y="13.8" width="2.2" height="4.6" rx="1.1" />
      </g>

      <g className="cat-frame-b">
        <rect x="8.2" y="13.8" width="2.2" height="4.6" rx="1.1" transform="rotate(-28 9.3 14.2)" />
        <rect x="11.4" y="13.8" width="2.2" height="4.6" rx="1.1" transform="rotate(26 12.5 14.2)" />
        <rect x="15.8" y="13.8" width="2.2" height="4.6" rx="1.1" transform="rotate(-26 16.9 14.2)" />
        <rect x="18.6" y="13.8" width="2.2" height="4.6" rx="1.1" transform="rotate(28 19.7 14.2)" />
      </g>
    </svg>
  )
}
