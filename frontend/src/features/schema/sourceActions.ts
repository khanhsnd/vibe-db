import type { Namespace } from '@/bridge/connections'
import { schema, type SourceKind, type SourceRef } from '@/bridge/schema'
import type { MenuItem } from '@/shared/ui/ContextMenu'
import { ICON } from '@/shared/ui/menuIcons'
import type { ChangeSchema, SchemaTarget } from '@/stores/connections'
import type { SchemaRequest } from '@/stores/schema'
import { SOURCE_NOUN, type SourceTarget } from '@/stores/tabs'

export interface SourceDeps {
  ask: (request: SchemaRequest) => void
  changeSchema: (profileId: string, target: SchemaTarget, change: ChangeSchema) => Promise<void>
  /** Opens the definition in a tab of its own, which is where it is edited. */
  openSource: (target: SourceTarget) => void
}

export interface SourceActions {
  /** The definition as the server keeps it, open for editing. */
  design: () => void
  /** A new one of these, named first and then opened on the engine's own skeleton. */
  create: (kind: SourceKind) => void
  remove: () => void
}

/** What dropping each kind costs the reader, which is the whole of what the confirmation says. */
const REMOVAL_COST: Record<SourceKind, string> = {
  view: 'The rows it read stay where they are; anything selecting from it stops working.',
  function: 'Anything that calls it stops working.',
  procedure: 'Anything that calls it stops working.',
  trigger: 'The rows it has already written stay; nothing fires on that table again.',
}

/**
 * What can be done to a view or a routine, wherever one is offered. It is `tableActions`' sibling
 * and follows the same two rules: nothing runs without being confirmed or filled in first, and
 * whatever asked is where a rejection is shown.
 *
 * The three kinds share every item because they differ in one word — `SOURCE_NOUN` — which is
 * exactly what differs in the statements behind them. What is *not* shared is where they are
 * offered at all: an engine whose views are not declared by a statement has no definition to open,
 * and says so with `hasSource` rather than being given a Design that could only fail.
 */
export function sourceActions(
  profileId: string,
  namespace: Namespace,
  ref: SourceRef,
  deps: SourceDeps,
): SourceActions {
  return {
    design: () => deps.openSource({ profileId, namespace, ref }),

    // The name is asked for **before** the tab opens, because the skeleton carries it: a
    // declaration addresses the object it declares, and there is no second box on the tab
    // repeating what the statement already says. Nothing is sent here — the reader's Save is
    // what reaches a server.
    //
    // The table comes from whatever this action was built for, which is how a New Trigger knows
    // what it fires on: it is offered on a table and on a trigger, and both already name one.
    create: (kind) =>
      deps.ask({
        kind: 'prompt',
        title: `New ${SOURCE_NOUN[kind]}`,
        label: 'Name',
        value: '',
        submitLabel: 'Create',
        onSubmit: async (name) => {
          deps.openSource({
            profileId,
            namespace,
            ref: { kind, name, arguments: '', table: ref.table },
            creating: true,
          })
        },
      }),

    remove: () =>
      deps.ask({
        kind: 'confirm',
        title: `Delete ${SOURCE_NOUN[ref.kind]}`,
        message: `Drop ${ref.name}? ${REMOVAL_COST[ref.kind]}`,
        submitLabel: 'Delete',
        danger: true,
        // The reference travels whole rather than by its name: the arguments are part of what
        // names a PostgreSQL function and the table is part of what names a trigger there — the
        // same reason a tab is identified by all of them.
        onSubmit: () =>
          deps.changeSchema(profileId, { namespace }, (sessionId) =>
            schema.dropSource(sessionId, namespace, ref),
          ),
      }),
  }
}

/**
 * The New items a folder offers, which is one per kind that folder holds: `Views` offers a view,
 * and `Routines` offers a function *and* a procedure — no engine has a folder for one of them
 * alone. Neither is offered where this engine's objects are not declared by a statement.
 */
export function newSourceItems(
  kinds: readonly SourceKind[],
  create: (kind: SourceKind) => void,
): MenuItem[] {
  return kinds.map((kind) => ({
    label: `New ${SOURCE_NOUN[kind]}…`,
    icon: ICON.add,
    onSelect: () => create(kind),
  }))
}
