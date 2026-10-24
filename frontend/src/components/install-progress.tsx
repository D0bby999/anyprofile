// SPDX-License-Identifier: AGPL-3.0-or-later
import type { EngineProgress } from '../api'
import { t } from '../i18n'

const mb = (n: number) => `${Math.round(n / 1048576)} MB`

/** Download progress of an engine install, with percent and megabytes. Once
 *  every byte is in, the install is checking the signature and unpacking,
 *  which takes a while for big archives: it says so instead of a still 100%. */
export function InstallProgress({ progress }: { progress: EngineProgress }) {
  const pct = progress.total > 0 ? Math.round((progress.done / progress.total) * 100) : 0
  const unpacking = progress.total > 0 && progress.done >= progress.total
  return (
    <div className="field">
      <span className="hint" aria-live="polite">{unpacking ? t.engines.unpacking
        : t.engines.installing(pct, progress.total > 0 ? `${mb(progress.done)} / ${mb(progress.total)}` : '')}</span>
      <div className="progress" role="progressbar" aria-valuenow={pct} aria-valuemin={0} aria-valuemax={100}><div style={{ width: `${pct}%` }} /></div>
    </div>
  )
}
