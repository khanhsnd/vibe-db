import * as api from '@wailsjs/go/app/UpdateAPI'
import type { app } from '@wailsjs/go/models'

import { call } from './errors'

export type UpdateInfo = app.UpdateInfo

/** The installed app decides availability and, on Windows, verifies the installer before launch. */
export const updates = {
  check: (): Promise<UpdateInfo> => call(() => api.Check()),
  installWindows: (): Promise<void> => call(() => api.InstallWindowsUpdate()),
  openRelease: (): Promise<void> => call(() => api.OpenRelease()),
}
