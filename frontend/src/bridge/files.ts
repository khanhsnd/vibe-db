import * as api from '@wailsjs/go/app/SystemAPI'

import { call } from './errors'

/**
 * A path out of the host's own file picker, and `''` for a dialog the reader dismissed.
 *
 * `title` says what is being picked and doubles as the name of the filter `pattern` is offered
 * under; All files sits behind it always, and is the whole list where no pattern is given — a
 * private key or a `known_hosts` file usually carries no extension at all.
 *
 * Nothing is read: this asks the machine for a path, and what is at it is opened at connect time.
 */
export const files = {
  choose: (title: string, pattern = ''): Promise<string> =>
    call(() => api.ChooseFile(title, pattern)),
}
