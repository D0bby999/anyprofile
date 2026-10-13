// SPDX-License-Identifier: AGPL-3.0-or-later
import { service } from '../api'
import { t } from '../i18n'

/** Fingerprint verdict chip, from the measurement recorded at the last launch. */
export function FpChip({ row }: { row: service.ProfileRow }) {
  if (!row.fpVerdict) return null
  if (row.fpVerdict === 'match') return <span className="chip ok"><span className="dot" />{t.fp.ok}</span>
  if (row.fpVerdict === 'drift') {
    return <span className="chip danger" title={t.fp.diff(row.fpDiff.join(', '))}><span className="dot" />{t.fp.drift}</span>
  }
  if (row.fpVerdict === 'baseline') return <span className="chip"><span className="dot" />{t.fp.baseline}</span>
  if (row.fpVerdict === 'unchecked') return <span className="chip warn"><span className="dot" />{t.fp.unchecked}</span>
  return null
}
