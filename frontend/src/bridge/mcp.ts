import * as api from '@wailsjs/go/app/SystemAPI'

import { call } from './errors'

/** How an AI agent reaches this app — see docs/features/mcp.md. */
export const mcp = {
  /** The line that registers this copy of the app with Claude Code, naming where it is installed. */
  command: (): Promise<string> => call(() => api.MCPCommand()),
}
