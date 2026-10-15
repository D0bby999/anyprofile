// SPDX-License-Identifier: AGPL-3.0-or-later
import { Api, service } from '../../api'
import { t } from '../../i18n'
import { errorText, relativeTime } from '../../lib/format'
import { machineLabel } from '../../lib/persona'
import { useConfirm } from '../confirm'
import { HealthPill } from '../health-pill'
import { useToast } from '../toast'

/** What was fixed at creation, and the last fingerprint measurement. */
export function IdentityGroup({ row }: { row: service.ProfileRow }) {
  const confirm = useConfirm()
  const toast = useToast()
  const reset = async () => {
    if (!(await confirm(t.confirm.resetFingerprint))) return
    try {
      await Api.ResetFingerprint(row.id)
    } catch (e) {
      toast.error(errorText(e))
    }
  }
  return (
    <section className="dgroup" aria-label={t.drawer.identity}>
      <h3>{t.drawer.identity}</h3>
      <dl className="facts selectable">
        <dt>{t.editor.persona}</dt><dd>{machineLabel(row)}</dd>
        <dt>{t.editor.engine}</dt><dd>{(t.engines.names[row.engine] ?? row.engine) + ' ' + row.engineVersion}</dd>
        <dt>{t.editor.seed}</dt><dd className="mono">{row.seedHex}</dd>
        <dt>{t.editor.created}</dt><dd>{relativeTime(row.createdAt)}</dd>
        <dt>{t.fp.title}</dt>
        <dd className="fpcell">
          <HealthPill row={row} />
          {(row.fpCheckedAt || row.fpVerdict) && (
            <span className="fpmeta">
              {row.fpCheckedAt && <span className="muted">{t.drawer.fingerprintChecked(relativeTime(row.fpCheckedAt))}</span>}
              {row.fpVerdict && <button type="button" className="linkbtn" onClick={reset}>{t.fp.reset}</button>}
            </span>
          )}
        </dd>
      </dl>
      <span className="hint">{t.editor.identityHint}</span>
    </section>
  )
}
