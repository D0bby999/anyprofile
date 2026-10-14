// SPDX-License-Identifier: AGPL-3.0-or-later
import { t } from '../i18n'
import { Dialog } from './dialog'

/** The list of keyboard shortcuts, opened with "?". */
export function ShortcutSheet({ onClose }: { onClose: () => void }) {
  return (
    <Dialog title={t.keys.title} icon="settings" onClose={onClose} footer={<button type="button" className="btn" autoFocus onClick={onClose}>{t.common.close}</button>}>
      <dl className="keys">
        {t.keys.rows.map(([k, what]) => (
          <div key={k} className="keyrow"><dt><kbd>{k}</kbd></dt><dd>{what}</dd></div>
        ))}
      </dl>
    </Dialog>
  )
}
