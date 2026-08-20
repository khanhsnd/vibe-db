import * as api from '@wailsjs/go/app/StoreAPI'
import * as system from '@wailsjs/go/app/SystemAPI'
import type { app } from '@wailsjs/go/models'
import { EventsOn } from '@wailsjs/runtime/runtime'

import type { ProxySettings, SSHSettings, TLSSettings } from './connections'
import { call } from './errors'

/**
 * An object store: a connection to one, what a bucket holds, and the transfers a reader has asked
 * for. It is `connections.ts`'s counterpart and shares three of its shapes — TLS, a bastion, a
 * proxy — because those are about the *connection* and mean the same thing whatever is at the far
 * end of it.
 *
 * The shapes are spelled out here rather than taken from `app.*` for the reason a profile's are:
 * Wails generates a **class** for any wire type holding a nested struct, and an object literal can
 * then never satisfy it. `wire` is the one place either crosses.
 */

/**
 * Which service a store *is*. It is a **preset and not an adapter**: every one of them speaks the
 * same protocol, so what a provider decides is three fields the reader would otherwise have to know
 * by heart — where it is, whether the bucket goes in the host or in the path, and what a region
 * means to it.
 *
 * `needsEndpoint` is why the list is worth having at all: AWS's address can be worked out from a
 * region and nobody else's can.
 */
export interface ProviderInfo {
  value: string
  label: string
  /** The colour its chip is drawn in — a `--store-*` token and never a hex value in a component. */
  colour: string
  needsEndpoint: boolean
  /** What the endpoint box starts with, as a hint rather than a value: nothing is filled in for the
   *  reader, because an account id or a region in the middle of it is theirs to type. */
  endpointHint: string
  /** Whether the bucket is named in the path rather than as a label of the host. */
  pathStyle: boolean
  /** The region such a service signs for when the profile names none, and `''` where it needs one. */
  region: string
  hint: string
}

export const PROVIDERS = [
  {
    value: 'aws',
    label: 'Amazon S3',
    colour: 'var(--store-aws)',
    needsEndpoint: false,
    endpointHint: 's3.eu-west-1.amazonaws.com',
    pathStyle: false,
    region: '',
    hint: 'The endpoint is worked out from the region. Name a region.',
  },
  {
    value: 'minio',
    label: 'MinIO',
    colour: 'var(--store-minio)',
    needsEndpoint: true,
    endpointHint: 'http://127.0.0.1:9000',
    pathStyle: true,
    region: 'us-east-1',
    hint: 'A MinIO endpoint names the bucket in the path. Say http:// out loud for one without TLS.',
  },
  {
    value: 'r2',
    label: 'Cloudflare R2',
    colour: 'var(--store-r2)',
    needsEndpoint: true,
    endpointHint: '<account>.r2.cloudflarestorage.com',
    pathStyle: false,
    region: 'auto',
    hint: 'R2 signs for the region “auto” and has no others.',
  },
  {
    value: 'spaces',
    label: 'DigitalOcean Spaces',
    colour: 'var(--store-spaces)',
    needsEndpoint: true,
    endpointHint: 'fra1.digitaloceanspaces.com',
    pathStyle: false,
    region: '',
    hint: 'The region is the first label of the endpoint — fra1, nyc3, sgp1.',
  },
  {
    value: 'wasabi',
    label: 'Wasabi',
    colour: 'var(--store-wasabi)',
    needsEndpoint: true,
    endpointHint: 's3.eu-central-1.wasabisys.com',
    pathStyle: false,
    region: '',
    hint: '',
  },
  {
    value: 'gcs',
    label: 'Google Cloud Storage',
    colour: 'var(--store-gcs)',
    needsEndpoint: true,
    endpointHint: 'storage.googleapis.com',
    pathStyle: true,
    region: 'us-east-1',
    hint: 'The interoperability endpoint, with an HMAC key rather than a service account.',
  },
  {
    value: 'other',
    label: 'Other S3-compatible',
    colour: 'var(--store-other)',
    needsEndpoint: true,
    endpointHint: 'https://storage.example.com',
    pathStyle: true,
    region: 'us-east-1',
    hint: 'Ceph, SeaweedFS, Garage, Backblaze B2 — anything that speaks the S3 API.',
  },
] as const satisfies readonly ProviderInfo[]

export const DEFAULT_PROVIDER = PROVIDERS[0].value

/**
 * What a provider decides, read through one function so that no component ever compares a provider's
 * *name* against anything — the rule `engineInfo` sets for an engine.
 */
export function providerInfo(value: string): ProviderInfo {
  // Anything unrecognised reads as the generic one rather than as nothing: a profile written by a
  // later version naming a provider this one has not heard of still speaks the same protocol.
  return PROVIDERS.find((one) => one.value === value) ?? OTHER_PROVIDER
}

const OTHER_PROVIDER: ProviderInfo = PROVIDERS[PROVIDERS.length - 1] as ProviderInfo

/** How the service authenticates the reader: a key pair they hold, or the machine's own identity. */
export const STORE_AUTH = [
  {
    value: 'key',
    label: 'Access key',
    hint: 'An access key id and its secret. The secret is kept in this machine’s credential store.',
  },
  {
    value: 'environment',
    label: 'AWS environment',
    hint:
      'The credentials the machine’s AWS tools already agree on, read at the moment of connecting ' +
      'and never stored — which is what makes an SSO login or an assumed role work here.',
  },
] as const

export interface Store {
  id: string
  name: string
  provider: string
  environment: string
  colour: string
  access: string
  endpoint: string
  region: string
  bucket: string
  pathStyle: boolean
  auth: string
  accessKeyId: string
  tls: TLSSettings
  ssh: SSHSettings
  proxy: ProxySettings
  saveSecret: boolean
  /** That a secret is stored, never what it is — the rule `hasPassword` follows. */
  hasSecret: boolean
  hasSshSecret: boolean
  hasProxySecret: boolean
  target: string
}

export interface StoreInput extends Omit<Store, 'hasSecret' | 'hasSshSecret' | 'hasProxySecret' | 'target'> {
  secretKey: string
  sshSecret: string
  proxySecret: string
}

/** One action's answer: what it is called, how dangerous it is, and what this connection will do. */
export interface ObjectRule {
  verb: string
  hazard: string
  /** Whether it has to be answered for — `Environment.NeedsConfirmation`, decided by the Go side. */
  confirm: boolean
  /** Whether it will not be sent at all — a read-only connection. */
  refused: boolean
}

/**
 * What each thing a reader can do to a bucket costs. It arrives **once, with the session**, because
 * it is a pure function of the profile: eight actions, one hazard each, one environment. Nothing on
 * this side ever re-derives it, which is the rule a statement's own `confirm` already follows.
 */
export interface ObjectRules {
  upload: ObjectRule
  delete: ObjectRule
  deleteFolder: ObjectRule
  emptyBucket: ObjectRule
  deleteBucket: ObjectRule
  copy: ObjectRule
  move: ObjectRule
  createBucket: ObjectRule
  /** Changing what an object's headers say, which is a **write**: the request rewrites the object. */
  headers: ObjectRule
}

export interface StoreConnection {
  sessionId: string
  profileId: string
  name: string
  provider: string
  bucket: string
  readOnly: boolean
  rules: ObjectRules
}

export interface BucketRow {
  name: string
  createdAt: string
  region: string
}

export interface ObjectRow {
  /** The whole key, which is what every action is addressed to. */
  key: string
  /** What the grid draws — the tail inside a folder, and the whole key in a search's answer. */
  name: string
  folder: boolean
  size: number
  modifiedAt: string
  etag: string
  storageClass: string
}

export interface ObjectPage {
  bucket: string
  prefix: string
  rows: ObjectRow[]
  /** Continues the listing. Empty is the last page — see `ListObjectsInput.token`. */
  next: string
  folders: number
  objects: number
  bytes: number
}

export interface ListObjectsInput {
  bucket: string
  prefix: string
  /** The service's own continuation token, opaque here. A page is forward only, so going back is
   *  the tab's own stack of these rather than a re-read of the bucket. */
  token: string
  limit: number
  /** Everything under the prefix rather than the folder alone — what Flatten asks for. */
  flat: boolean
}

export interface ObjectDetail {
  key: string
  size: number
  modifiedAt: string
  etag: string
  storageClass: string
  contentType: string
  /** Whether the service's tag is a plain checksum of the bytes — false for a multipart upload. */
  verifiable: boolean
}

/** One line of what the service said about an object. Both halves are verbatim. */
export interface ObjectHeader {
  name: string
  value: string
}

/**
 * Everything a `Head` answers with. **One request feeds two panes** — Properties and Header — because
 * they are two readings of one answer, and asking twice would be one request too many.
 */
export interface ObjectFacts {
  detail: ObjectDetail
  /** The whole answer, sorted by name: the order the service sent them in is not preserved. */
  headers: ObjectHeader[]
  /** What the *uploader* attached, with the `x-amz-meta-` prefix taken off. */
  metadata: ObjectHeader[]
  /**
   * The header names a service will actually take — six, fixed by the protocol. It arrives from the
   * Go side rather than being spelled here, so the pane and the request that writes them cannot come
   * to disagree: a row not named here is the **service's own** and there is no request that changes
   * it.
   */
  settable: string[]
}

/**
 * The whole set an object's headers are to become. It is a **set and not an edit**, because the
 * request underneath is a copy of the object onto itself: a header left out of this is a header the
 * object no longer has — which is why the pane stages what is typed and applies it in one press.
 */
export interface SetHeadersInput {
  /** The settable six, by their HTTP names. */
  system: Record<string, string>
  /** The `x-amz-meta-` pairs, without the prefix. */
  meta: Record<string, string>
}

export interface ObjectPermission {
  grantee: string
  kind: string
  allow: string
}

/**
 * An object's access-control policy, **read and never written**: writing one is a whole grammar of
 * canned names and grant lists, and letting a reader edit half of it would quietly drop the other
 * half. A service that has not got them answers its own refusal, which the pane shows — an empty
 * table would read as "nobody may do anything", which is a different and wrong sentence.
 */
export interface ObjectPermissions {
  owner: string
  grants: ObjectPermission[]
  /** A grant that opens the object to anyone at all. Worked out by the Go side, and drawn loudly. */
  public: boolean
}

export interface ObjectTag {
  name: string
  value: string
}

/** One version, newest first. `deleted` is a **delete marker** rather than a version with bytes. */
export interface ObjectVersion {
  versionId: string
  latest: boolean
  deleted: boolean
  size: number
  modifiedAt: string
  etag: string
  storageClass: string
}

/**
 * One thing **this app** did about an object. It is not the service's own access log and the pane
 * says so: that is a bucket setting writing log objects into another bucket, usually off, and a
 * client showing one would be showing somebody else's gigabytes. What a reader wants from a Log tab
 * is *what did this program just ask for, and what came back* — which only this side can answer.
 */
export interface ObjectLogEntry {
  at: string
  action: string
  key: string
  /** `ok`, or the code the UI would have switched on — `FORBIDDEN`, `UNKNOWN_OBJECT`. */
  outcome: string
  detail: string
  ms: number
}

export interface ObjectPreview {
  key: string
  contentType: string
  size: number
  text: string
  /** A base64 data URL, for the one kind of object worth drawing rather than reading. */
  data: string
  /** The object is longer than what is here. A preview must never be taken for the object. */
  truncated: boolean
}

/**
 * How the text in the search box is matched against a key. They are `domain.TextMatch`'s own modes —
 * the same four a Find in Schema takes — so `Contains` means of a key exactly what it means of a
 * column name.
 *
 * `prefix` is the one worth reaching for: it is the **only** thing a service can be asked about a
 * name, so a search spelled that way narrows at the far end and reads almost nothing.
 */
export const OBJECT_SEARCH_MODES = [
  {
    value: 'contains',
    label: 'Name contains',
    hint: 'Anywhere in the key, folders included. Reads every key under the folder.',
  },
  {
    value: 'prefix',
    label: 'Key starts with',
    hint:
      'The fast one — the service does the narrowing. The text must be the start of the whole key, ' +
      'so a file name alone will not find it: “private/77/a.docx” matches and “a.docx” does not.',
  },
  { value: 'word', label: 'Name holds the word', hint: 'Every key under the folder is read.' },
  {
    value: 'regex',
    label: 'Regular expression',
    hint: 'One anchored with ^ narrows at the service; anything else reads every key.',
  },
] as const

/**
 * **Contains**, and the reason is the one thing a search box must never do: answer "nothing matches"
 * about an object that is right there.
 *
 * It was `prefix` first, because that is the mode a service can narrow and therefore the quick one.
 * But a key is a *path* — `private/77/b586….docx` — and a reader pasting a file name into a box means
 * *find this file*, not *find keys that begin with this*. So the quick mode found nothing for the
 * commonest thing anybody types, which is a worse failure than a slow answer: one of them costs
 * seconds and the other sends a reader off believing their object has gone.
 *
 * `prefix` is still a press away, and its hint says exactly what it is for. Speed is the reader's to
 * choose; being right is not.
 */
export const DEFAULT_SEARCH_MODE = 'contains'

export interface SearchObjectsInput {
  bucket: string
  prefix: string
  text: string
  mode: string
  caseInsensitive: boolean
  minSize: number
  maxSize: number
  since: string
  until: string
  hits: number
  scan: number
  runId: string
}

export interface ObjectSearchResult {
  rows: ObjectRow[]
  scanned: number
  /** What the *service* was narrowed by, which is why the search was quick. It is shown. */
  prefix: string
  whole: boolean
  more: boolean
}

/** What to do about a file that is already there. The default loses nothing. */
export const COLLISIONS = [
  { value: 'skip', label: 'Skip', hint: 'Leave the file that is there.' },
  { value: 'newer', label: 'Only if newer', hint: 'Overwrite where the object is newer — a sync.' },
  { value: 'rename', label: 'Keep both', hint: 'Write beside it as “name (2)”.' },
  { value: 'overwrite', label: 'Overwrite', hint: 'Replace the file that is there.' },
] as const

export const DEFAULT_COLLISION = COLLISIONS[0].value

/**
 * One object a reader ticked, with the three facts **the listing they ticked it in already carried**.
 * They are sent rather than asked for again, and each earns its place: the size is what the progress
 * bar's total is made of, the tag is what a finished download is verified against, and the clock is
 * what `Only if newer` compares a file's against.
 */
export interface DownloadObject {
  key: string
  size: number
  etag: string
  modifiedAt: string
}

export interface DownloadInput {
  bucket: string
  keys: DownloadObject[]
  prefixes: string[]
  into: string
  under: string
  collision: string
}

/** A grid row as the thing a download is addressed to, which is the one place the two are joined. */
export function downloadObject(row: ObjectRow): DownloadObject {
  return { key: row.key, size: row.size, etag: row.etag, modifiedAt: row.modifiedAt }
}

export interface UploadInput {
  bucket: string
  prefix: string
  paths: string[]
}

export type TransferState = 'waiting' | 'running' | 'done' | 'failed' | 'partial' | 'cancelled'

export interface TransferJob {
  id: string
  kind: 'download' | 'upload'
  bucket: string
  label: string
  root: string
  state: TransferState
  files: number
  done: number
  failed: number
  skipped: number
  bytes: number
  moved: number
  /**
   * The listing that finds the files has not finished, so `files` and `bytes` are still climbing.
   * While it is true the panel draws a **count and not a percentage**: a bar drawn from a total that
   * is still moving is a bar that goes backwards.
   */
  counting: boolean
  rate: number
  detail: string
  startedAt: string
  endedAt: string
}

export interface TransferFile {
  jobId: string
  key: string
  path: string
  size: number
  moved: number
}

export interface TransferReport {
  jobs: TransferJob[]
  files: TransferFile[]
  active: number
}

export interface RemoveResult {
  deleted: number
  /** What the service would not remove. It is not a failure — see `RemoveResult` on the Go side. */
  refused: string[]
}

export interface CopyResult {
  objects: number
  bytes: number
}

export function finished(state: TransferState): boolean {
  return state === 'done' || state === 'failed' || state === 'partial' || state === 'cancelled'
}

/** What a store profile starts as: the first provider, a key pair, and a certificate that is checked. */
export function newStoreInput(): StoreInput {
  const provider = providerInfo(DEFAULT_PROVIDER)
  return {
    id: '',
    name: '',
    provider: provider.value,
    environment: '',
    colour: '',
    access: '',
    endpoint: '',
    region: '',
    bucket: '',
    pathStyle: provider.pathStyle,
    auth: 'key',
    accessKeyId: '',
    secretKey: '',
    // A store is reached over https, so the strictest of the five is the default — the one place
    // this app picks a TLS mode rather than leaving it at the engine's own.
    tls: { mode: 'verify-full', rootCert: '', clientCert: '', clientKey: '', serverName: '' },
    ssh: {
      enabled: false,
      host: '',
      port: 22,
      user: '',
      auth: 'key',
      keyPath: '',
      knownHosts: '',
      strictHostKey: true,
    },
    sshSecret: '',
    proxy: { enabled: false, kind: 'socks5', host: '', port: 1080, user: '' },
    proxySecret: '',
    saveSecret: true,
  }
}

export function toStoreInput(store: Store): StoreInput {
  return {
    id: store.id,
    name: store.name,
    provider: store.provider,
    environment: store.environment,
    colour: store.colour,
    access: store.access,
    endpoint: store.endpoint,
    region: store.region,
    bucket: store.bucket,
    pathStyle: store.pathStyle,
    auth: store.auth,
    accessKeyId: store.accessKeyId,
    // No secret ever comes back out of the store — `hasSecret` is how the dialog says one is kept.
    secretKey: '',
    tls: { ...store.tls },
    ssh: { ...store.ssh },
    proxy: { ...store.proxy },
    sshSecret: '',
    proxySecret: '',
    saveSecret: store.saveSecret,
  }
}

const wire = <T>(value: unknown) => value as T

export const objects = {
  listStores: (): Promise<Store[]> => call(() => api.ListStores()) as unknown as Promise<Store[]>,
  saveStore: (input: StoreInput): Promise<Store> =>
    call(() => api.SaveStore(wire<app.StoreInput>(input))) as unknown as Promise<Store>,
  deleteStore: (id: string) => call(() => api.DeleteStore(id)),
  testStore: (input: StoreInput) => call(() => api.TestStore(wire<app.StoreInput>(input))),

  connect: (profileId: string, secretKey: string): Promise<StoreConnection> =>
    call(() => api.ConnectStore(profileId, secretKey)) as unknown as Promise<StoreConnection>,
  disconnect: (sessionId: string) => call(() => api.DisconnectStore(sessionId)),

  listBuckets: (sessionId: string): Promise<BucketRow[]> =>
    call(() => api.ListBuckets(sessionId)) as unknown as Promise<BucketRow[]>,
  listObjects: (sessionId: string, input: ListObjectsInput): Promise<ObjectPage> =>
    call(() =>
      api.ListObjects(sessionId, wire<app.ListObjectsInput>(input)),
    ) as unknown as Promise<ObjectPage>,
  head: (sessionId: string, bucket: string, key: string): Promise<ObjectFacts> =>
    call(() => api.HeadObject(sessionId, bucket, key)) as unknown as Promise<ObjectFacts>,
  /** Answers with the object read **again**, because the write rewrote it — see `SetHeadersInput`. */
  setHeaders: (
    sessionId: string,
    bucket: string,
    key: string,
    input: SetHeadersInput,
  ): Promise<ObjectFacts> =>
    call(() =>
      api.SetObjectHeaders(sessionId, bucket, key, wire<app.SetHeadersInput>(input)),
    ) as unknown as Promise<ObjectFacts>,
  permissions: (sessionId: string, bucket: string, key: string): Promise<ObjectPermissions> =>
    call(() => api.ObjectPermissions(sessionId, bucket, key)) as unknown as Promise<ObjectPermissions>,
  tags: (sessionId: string, bucket: string, key: string): Promise<ObjectTag[]> =>
    call(() => api.ObjectTags(sessionId, bucket, key)) as unknown as Promise<ObjectTag[]>,
  versions: (sessionId: string, bucket: string, key: string): Promise<ObjectVersion[]> =>
    call(() => api.ObjectVersions(sessionId, bucket, key)) as unknown as Promise<ObjectVersion[]>,
  /** What this app did about one object. It asks the service nothing — see `ObjectLogEntry`. */
  log: (bucket: string, key: string): Promise<ObjectLogEntry[]> =>
    call(() => api.ObjectLog(bucket, key)) as unknown as Promise<ObjectLogEntry[]>,
  preview: (sessionId: string, bucket: string, key: string): Promise<ObjectPreview> =>
    call(() => api.PreviewObject(sessionId, bucket, key)) as unknown as Promise<ObjectPreview>,
  search: (sessionId: string, input: SearchObjectsInput): Promise<ObjectSearchResult> =>
    call(() =>
      api.SearchObjects(sessionId, wire<app.SearchObjectsInput>(input)),
    ) as unknown as Promise<ObjectSearchResult>,
  stopRun: (runId: string) => call(() => api.StopObjectRun(runId)),
  presign: (sessionId: string, bucket: string, key: string, seconds: number): Promise<string> =>
    call(() => api.PresignObject(sessionId, bucket, key, seconds)),

  remove: (sessionId: string, bucket: string, keys: string[]): Promise<RemoveResult> =>
    call(() => api.DeleteObjects(sessionId, bucket, keys)) as unknown as Promise<RemoveResult>,
  removeFolder: (
    sessionId: string,
    bucket: string,
    prefix: string,
    runId: string,
  ): Promise<RemoveResult> =>
    call(() =>
      api.DeleteFolder(sessionId, bucket, prefix, runId),
    ) as unknown as Promise<RemoveResult>,
  copy: (
    sessionId: string,
    bucket: string,
    from: string,
    toBucket: string,
    to: string,
    runId: string,
  ): Promise<CopyResult> =>
    call(() =>
      api.CopyObjects(sessionId, bucket, from, toBucket, to, runId),
    ) as unknown as Promise<CopyResult>,
  move: (sessionId: string, bucket: string, from: string, to: string) =>
    call(() => api.MoveObject(sessionId, bucket, from, to)),
  newFolder: (sessionId: string, bucket: string, prefix: string, name: string) =>
    call(() => api.NewFolder(sessionId, bucket, prefix, name)),
  makeBucket: (sessionId: string, name: string) => call(() => api.MakeBucket(sessionId, name)),
  removeBucket: (sessionId: string, name: string) => call(() => api.RemoveBucket(sessionId, name)),

  download: (sessionId: string, input: DownloadInput): Promise<string> =>
    call(() => api.DownloadObjects(sessionId, wire<app.DownloadInput>(input))),
  upload: (sessionId: string, input: UploadInput): Promise<string> =>
    call(() => api.UploadFiles(sessionId, wire<app.UploadInput>(input))),
  transfers: (): Promise<TransferReport> =>
    call(() => api.Transfers()) as unknown as Promise<TransferReport>,
  cancelTransfer: (jobId: string) => call(() => api.CancelTransfer(jobId)),
  cancelTransfers: () => call(() => api.CancelTransfers()),
  clearTransfers: () => call(() => api.ClearTransfers()),
  revealFolder: (path: string) => call(() => api.RevealFolder(path)),

  /** The host's own pickers. Where a download goes and what an upload sends are facts about *this*
   *  machine, which is why they come from `SystemAPI` rather than from a store. */
  chooseFolder: (title: string): Promise<string> => call(() => system.ChooseFolder(title)),
  chooseFiles: (title: string): Promise<string[]> => call(() => system.ChooseFiles(title)),
}

/**
 * The transfer report, pushed while anything is moving. It is an event and not a poll for the reason
 * a scheduled job's firing is: a reader watching a forty-gigabyte download deserves a number that
 * moves, and nothing about the bytes crosses the bridge — only how far they have got.
 */
export const EVENT_TRANSFERS = 'transfers:report'

/**
 * It is the **one** subscription in this app that is made whether or not the reader has opened
 * anything: the transfer dock is mounted from the start, because a download has to be watchable the
 * moment one exists. Every other `EventsOn` here is reached from a view somebody opened.
 *
 * That is why this one checks the runtime is there first. Under a plain `pnpm dev` — Vite without
 * Wails — `window.runtime` does not exist, and an unguarded subscription at mount would take the
 * whole window down rather than merely failing the backend calls, which is the promise
 * `docs/development.md` makes about that server.
 */
export function onTransfers(handle: (report: TransferReport) => void): () => void {
  // The runtime is injected by the host rather than imported, so it is asked about and not typed.
  if (!('runtime' in globalThis)) return () => {}
  return EventsOn(EVENT_TRANSFERS, (report: TransferReport) => handle(report))
}
