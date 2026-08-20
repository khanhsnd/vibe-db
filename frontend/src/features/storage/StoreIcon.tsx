import { providerInfo } from '@/bridge/objects'
import { cn } from '@/lib/cn'

/**
 * Which service a store connection speaks, as a chip — `EngineIcon`'s counterpart, and drawn the
 * same way for the same reason: at sixteen pixels a logo is a **shape**, so what is drawn is the
 * thing the service *is* rather than a badge nobody would recognise anyway.
 *
 * The shape is one shape: a **bucket**, because that is what every one of these holds and the
 * difference between them is not a difference a reader needs to see. What tells them apart is the
 * colour, which is a `--store-*` token and never a hex value here.
 */
const CHIPS: Record<string, string> = {
  aws: 'bg-store-aws',
  minio: 'bg-store-minio',
  r2: 'bg-store-r2',
  spaces: 'bg-store-spaces',
  wasabi: 'bg-store-wasabi',
  gcs: 'bg-store-gcs',
  other: 'bg-store-other',
}

export function StoreIcon({ provider, className }: { provider: string; className?: string }) {
  const info = providerInfo(provider)

  return (
    <span
      role="img"
      aria-label={info.label}
      title={info.label}
      className={cn(
        'grid h-4 w-4 shrink-0 place-items-center rounded-sm p-px text-on-accent',
        CHIPS[info.value] ?? CHIPS.other,
        className,
      )}
    >
      {/* A pail: two sides tapering in, a rim across the top, a handle over it. Drawn rather than
          vendored, because a bucket at sixteen pixels is four lines. */}
      <svg viewBox="0 0 16 16" className="h-full w-full" fill="none" aria-hidden>
        <path
          d="M3 5.5h10l-1.1 7.2a.9.9 0 0 1-.9.8H5a.9.9 0 0 1-.9-.8L3 5.5Z"
          fill="currentColor"
        />
        <path
          d="M5 5.2a3 3 0 0 1 6 0"
          stroke="currentColor"
          strokeWidth="1.3"
          strokeLinecap="round"
        />
      </svg>
    </span>
  )
}
