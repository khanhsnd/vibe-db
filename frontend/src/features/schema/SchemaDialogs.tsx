import { useSchemaStore } from '@/stores/schema'

import { ConfirmDialog } from './ConfirmDialog'
import { PromptDialog } from './PromptDialog'

/**
 * The two schema requests that stay modal: a rename, which is one box, and a confirmation, which
 * has to be answered before anything else happens. Every request with a form in it opens in the
 * side panel instead — see `SchemaPanel`.
 */
export function SchemaDialogs() {
  const request = useSchemaStore((s) => s.request)
  const dismiss = useSchemaStore((s) => s.dismiss)

  if (request?.kind === 'prompt') return <PromptDialog request={request} onClose={dismiss} />
  if (request?.kind === 'confirm') return <ConfirmDialog request={request} onClose={dismiss} />
  return null
}
