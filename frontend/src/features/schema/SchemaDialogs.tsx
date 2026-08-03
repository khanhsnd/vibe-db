import { useSchemaStore } from '@/stores/schema'

import { ConfirmDialog } from './ConfirmDialog'
import { DatabaseDialog } from './DatabaseDialog'
import { PromptDialog } from './PromptDialog'

/**
 * The schema requests that stay modal: a rename, which is one box; a confirmation, which has to
 * be answered before anything else happens; and a new database, which has no tab of its own to
 * open. Everything else with a form in it is a designer tab, which has the room to work in.
 */
export function SchemaDialogs() {
  const request = useSchemaStore((s) => s.request)
  const dismiss = useSchemaStore((s) => s.dismiss)

  if (request?.kind === 'prompt') return <PromptDialog request={request} onClose={dismiss} />
  if (request?.kind === 'confirm') return <ConfirmDialog request={request} onClose={dismiss} />
  if (request?.kind === 'database') return <DatabaseDialog request={request} onClose={dismiss} />
  return null
}
