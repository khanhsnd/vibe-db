import * as api from '@wailsjs/go/app/SystemAPI'

/**
 * The font families installed on this machine, sorted, for the picker in Settings. The list is the
 * host's to give: a run outside the desktop shell answers with nothing, and the picker then offers
 * only the families the stylesheet ships with.
 */
export async function listFonts(): Promise<string[]> {
  try {
    return await api.ListFonts()
  } catch {
    return []
  }
}
