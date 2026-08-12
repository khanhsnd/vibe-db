import type { ReactNode } from 'react'
import { Bookmark, ListTree, Loader2, Play, Square, WrapText } from 'lucide-react'

import type { Namespace } from '@/bridge/connections'
import { DEFAULT_QUERY_ROWS, QUERY_ROW_LIMITS } from '@/bridge/rows'
import { NamespacePicker } from '@/features/connections/NamespacePicker'
import { TransactionMenu } from '@/features/results/TransactionMenu'
import { Button } from '@/shared/ui/Button'
import { RowLimit } from '@/shared/ui/RowLimit'

interface QueryToolbarProps {
  /**
   * The connection the statements run over, and its engine. A manual transaction is a fact of the
   * connection rather than of this tab, so the menu is drawn from the session — see
   * `TransactionMenu`, which draws nothing on an engine that has none.
   */
  sessionId: string | null
  engine: string
  databases: string[]
  schemas: string[]
  namespace: Namespace
  /** Whether this engine has a schema below a database, and so a second picker. */
  hasSchemas: boolean
  rowLimit: number
  running: boolean
  canRun: boolean
  /**
   * Whether the buffer holds anything at all. It is not `canRun`: running needs a connection and
   * naming a statement does not, so the two buttons are dead for different reasons.
   */
  hasText: boolean
  onNamespaceChange: (namespace: Namespace) => void
  onRowLimitChange: (limit: number) => void
  onExecute: () => void
  onExplain: () => void
  onFormat: () => void
  /** Names what is in the editor and keeps it — see the Snippets pane below. */
  onSaveSnippet: () => void
  onStop: () => void
}

export function QueryToolbar({
  sessionId,
  engine,
  databases,
  schemas,
  namespace,
  hasSchemas,
  rowLimit,
  running,
  canRun,
  hasText,
  onNamespaceChange,
  onRowLimitChange,
  onExecute,
  onExplain,
  onFormat,
  onSaveSnippet,
  onStop,
}: QueryToolbarProps) {
  return (
    <div
      role="toolbar"
      className="flex h-9 shrink-0 items-center gap-1.5 border-b border-line bg-bar px-2"
    >
      <NamespacePicker
        databases={databases}
        schemas={schemas}
        namespace={namespace}
        hasSchemas={hasSchemas}
        onChange={onNamespaceChange}
      />

      <span className="mx-1 h-4 w-px bg-line" />

      <Button
        variant="primary"
        icon={<Play size={12} fill="currentColor" />}
        disabled={!canRun || running}
        onClick={onExecute}
      >
        Execute
      </Button>
      <Shortcut>F5</Shortcut>

      <Button
        variant="default"
        icon={<ListTree size={12} />}
        disabled={!canRun || running}
        onClick={onExplain}
      >
        Explain
      </Button>

      {/* Stop is the danger colour only while there is something to stop: a red button standing
          over an idle editor claims a statement is running when none is. */}
      <Button
        variant={running ? 'danger' : 'default'}
        icon={<Square size={11} fill="currentColor" />}
        disabled={!running}
        onClick={onStop}
      >
        Stop
      </Button>

      <span className="mx-1 h-4 w-px bg-line" />

      {/* Laying a script out is the engine's — where a statement ends and what a quote is are
          lexical — so it needs the connection exactly as Execute does. */}
      <Button
        variant="default"
        icon={<WrapText size={12} />}
        disabled={!canRun || running}
        onClick={onFormat}
      >
        Format
      </Button>
      <Shortcut>Ctrl+Shift+F</Shortcut>

      {/* Naming a statement needs no server: a snippet is text this machine keeps, and one is worth
          saving whether or not the connection it was written against is still open. So this is alive
          on an empty session and dead only on an empty buffer. */}
      <Button
        variant="default"
        icon={<Bookmark size={12} />}
        disabled={!hasText}
        onClick={onSaveSnippet}
      >
        Save Snippet
      </Button>

      {sessionId !== null ? (
        <TransactionMenu sessionId={sessionId} namespace={namespace} engine={engine} />
      ) : null}

      {running ? (
        <span className="ml-1 flex items-center gap-1.5 text-xs text-muted">
          <Loader2 size={13} className="animate-spin text-accent" />
          Running…
        </span>
      ) : null}

      <div className="min-w-2 flex-1" />

      <div className="flex shrink-0 items-center text-xs text-muted">
        <RowLimit
          label="Limit"
          presets={QUERY_ROW_LIMITS}
          fallback={DEFAULT_QUERY_ROWS}
          value={rowLimit}
          onChange={onRowLimitChange}
        />
      </div>
    </div>
  )
}

/**
 * The key that does what the button beside it does, drawn as the key: a reader who is looking for
 * the shortcut finds a keycap where plain grey text reads as part of the label before it.
 */
function Shortcut({ children }: { children: ReactNode }) {
  return (
    <kbd className="mr-1 shrink-0 rounded border border-line bg-elevated px-1 py-px font-mono text-2xs text-faint">
      {children}
    </kbd>
  )
}

