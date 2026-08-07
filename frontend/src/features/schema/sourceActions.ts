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
            ref: { kind, name, arguments: '' },
            creating: true,
          })
        },
      }),

    remove: () =>
      deps.ask({
        kind: 'confirm',
        title: `Delete ${SOURCE_NOUN[ref.kind]}`,
        message:
          ref.kind === 'view'
            ? `Drop the view ${ref.name}? The rows it read stay where they are; ` +
              `anything selecting from it stops working.`
            : `Drop ${ref.name}? Anything that calls it stops working.`,
        submitLabel: 'Delete',
        danger: true,
        // A routine travels whole rather than by its name, the arguments being part of what names
        // one on PostgreSQL — the same reason its tab is identified by all three of them.
        onSubmit: () =>
          deps.changeSchema(profileId, { namespace }, (sessionId) =>
            ref.kind === 'view'
              ? schema.dropView(sessionId, namespace, ref.name)
              : schema.dropRoutine(sessionId, namespace, {
                  name: ref.name,
                  procedure: ref.kind === 'procedure',
                  arguments: ref.arguments,
                }),
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
