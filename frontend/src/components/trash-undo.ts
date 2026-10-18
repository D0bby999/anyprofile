// SPDX-License-Identifier: AGPL-3.0-or-later
import { Api } from '../api'
import { t } from '../i18n'
import type { ToastApi } from './toast'

/**
 * Moves profiles to the trash without asking (the trash gives them back)
 * and says so, with Undo. The ones moved before a failure are still
 * reported; the failure is thrown on.
 */
export async function trashWithUndo(ids: string[], toast: ToastApi, name: string, after?: () => void) {
  const done: string[] = []
  try {
    for (const id of ids) {
      await Api.TrashProfile(id)
      done.push(id)
    }
  } finally {
    if (done.length) {
      const undo = () => { Promise.all(done.map((id) => Api.RestoreProfile(id))).then(after).catch((e) => toast.error(String(e))) }
      toast.success(done.length === 1 && name ? t.profiles.trashedOne(name) : t.profiles.trashed(done.length), { label: t.common.undo, run: undo })
    }
    after?.()
  }
}
