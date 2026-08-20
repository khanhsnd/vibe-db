import type { ReactNode } from 'react'
import {
  Archive,
  Braces,
  Code2,
  Database,
  FileText,
  Film,
  Folder,
  Image,
  Music,
  Sheet,
  FileType,
  File,
  Lock,
  Package,
} from 'lucide-react'

import type { ObjectRow } from '@/bridge/objects'
import { cn } from '@/lib/cn'

/**
 * What a row of a bucket *is*, drawn ahead of its name.
 *
 * **The shape carries the kind and the colour carries the class**, which is the same division
 * `StoreIcon` makes and for the same reason: at thirteen pixels a logo is a shape, so a reader picks
 * a picture out of a list before they read a word of it. There are two colours and not fifteen — a
 * folder's and an object's — because `--kind-*` is one token per kind of thing the app holds, and
 * inventing a token per file extension would be inventing a fifteenth kind of thing.
 *
 * A **folder** is the one row here that is not an object at all: it stands for everything under it,
 * so it is drawn as the folder it is and never as the first file inside it.
 */
export function objectIcon(row: ObjectRow): ReactNode {
  if (row.folder) {
    return <Folder size={13} className="shrink-0 text-kind-prefix" />
  }

  const Glyph = GLYPHS[extensionOf(row.name)] ?? File
  return <Glyph size={13} className="shrink-0 text-kind-object" />
}

/** The same glyph without a row, for the panel's own header and anywhere one is named. */
export function iconFor(name: string, folder: boolean, className?: string): ReactNode {
  const Glyph = folder ? Folder : (GLYPHS[extensionOf(name)] ?? File)
  return (
    <Glyph
      size={13}
      className={cn('shrink-0', folder ? 'text-kind-prefix' : 'text-kind-object', className)}
    />
  )
}

/** The extension, lower case, and empty for a name that has none — a `Dockerfile`, a `LICENSE`. */
function extensionOf(name: string): string {
  const at = name.lastIndexOf('.')
  return at <= 0 || at === name.length - 1 ? '' : name.slice(at + 1).toLowerCase()
}

/**
 * One glyph per **class** of object, not per extension: a reader is telling a picture from a
 * spreadsheet from an archive, and forty distinct outlines at this size would be forty smudges.
 * Anything unrecognised is a plain sheet, which is the honest answer — a listing knows the name and
 * nothing else about what is inside.
 */
const GLYPHS: Record<string, typeof File> = {
  // Pictures, which are also the one class the panel *draws* rather than reads.
  png: Image,
  jpg: Image,
  jpeg: Image,
  gif: Image,
  webp: Image,
  bmp: Image,
  svg: Image,
  ico: Image,
  avif: Image,
  heic: Image,
  tif: Image,
  tiff: Image,

  // Structured text, which is what most of a bucket turns out to be.
  json: Braces,
  jsonl: Braces,
  ndjson: Braces,
  yaml: Braces,
  yml: Braces,
  toml: Braces,
  xml: Code2,
  html: Code2,
  htm: Code2,
  css: Code2,
  js: Code2,
  ts: Code2,
  tsx: Code2,
  jsx: Code2,
  go: Code2,
  py: Code2,
  rb: Code2,
  rs: Code2,
  java: Code2,
  sh: Code2,
  ps1: Code2,

  // A table of rows, which is a spreadsheet whether or not Excel wrote it.
  csv: Sheet,
  tsv: Sheet,
  xlsx: Sheet,
  xls: Sheet,
  parquet: Sheet,
  avro: Sheet,
  orc: Sheet,

  // Prose, and the one thing every bucket has too much of.
  txt: FileText,
  log: FileText,
  md: FileText,
  rst: FileText,
  pdf: FileType,

  // Something packed, which is the class a reader most often wants to spot: it is what a download is
  // usually after and what a preview can say least about.
  zip: Archive,
  gz: Archive,
  tgz: Archive,
  bz2: Archive,
  xz: Archive,
  zst: Archive,
  tar: Archive,
  '7z': Archive,
  rar: Archive,

  // A database or a dump, which in a bucket is nearly always a backup.
  sql: Database,
  db: Database,
  sqlite: Database,
  bak: Database,
  dump: Database,

  mp4: Film,
  mov: Film,
  mkv: Film,
  webm: Film,
  avi: Film,

  mp3: Music,
  wav: Music,
  flac: Music,
  ogg: Music,
  m4a: Music,

  // Something built, and something sealed. The last one is worth its own shape: an object a reader
  // cannot open without a key is a different thing from one they merely have not opened.
  exe: Package,
  dll: Package,
  so: Package,
  dmg: Package,
  deb: Package,
  rpm: Package,
  whl: Package,
  jar: Package,
  gpg: Lock,
  pgp: Lock,
  asc: Lock,
  pem: Lock,
  key: Lock,
  crt: Lock,
  enc: Lock,
}
