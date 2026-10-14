// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'react'
import { Api, main, onEngineProgress, service, type EngineProgress } from '../api'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import { Icon } from './icons'

const mb = (n: number) => `${Math.round(n / 1048576)} MB`
const engineName = (id: string) => t.engines.names[id] ?? id

type Props = {
  engines: main.EngineInfo[]
  /** Number of profiles per engine id. */
  counts: Record<string, number>
  defaults: service.Defaults
  onChanged: () => void
}

/** One block per engine: status, install or reinstall, and the new-profile default. */
export function EngineSettings({ engines, counts, defaults, onChanged }: Props) {
  const [progress, setProgress] = useState<EngineProgress | null>(null)
  const [errors, setErrors] = useState<Record<string, string>>({})
  useEffect(() => onEngineProgress(setProgress), [])

  const install = async (id: string) => {
    setErrors((e) => ({ ...e, [id]: '' }))
    setProgress({ engine: id, done: 0, total: -1 })
    try {
      await Api.InstallEngine(id)
    } catch (e) {
      setErrors((x) => ({ ...x, [id]: errorText(e) }))
    } finally {
      setProgress(null)
      onChanged()
    }
  }

  const useForNew = async (id: string) => {
    setErrors((e) => ({ ...e, [id]: '' }))
    try {
      await Api.SetDefaults(service.Defaults.createFrom({ ...defaults, engine: id }))
      onChanged()
    } catch (e) {
      setErrors((x) => ({ ...x, [id]: errorText(e) }))
    }
  }

  return (
    <div className="engines">
      {engines.map((e) => {
        const busy = progress?.engine === e.engine
        const pct = busy && progress.total > 0 ? Math.round((progress.done / progress.total) * 100) : 0
        return (
          <div key={e.engine} className="engine" aria-busy={busy}>
            <div className="engine-head">
              <b>{engineName(e.engine)}</b>
              {e.engine === 'apostate' && <span className="chip accent">{t.engines.recommended}</span>}
              <span className="grow" />
              {e.problem
                ? <span className="chip danger"><span className="dot" />{t.engines.notInstalled}</span>
                : e.installed
                  ? <span className="chip ok"><span className="dot" />{t.engines.installed}</span>
                  : <span className="chip">{t.engines.notInstalled}</span>}
            </div>
            <p className="desc">{e.engine === 'apostate' ? t.engines.apostateDesc : t.engines.furyDesc}</p>
            <div className="engine-facts">
              <span>{t.engines.version(e.version)}</span>
              <span>{t.engines.chromium(e.chromium)}</span>
              <span>{t.engines.usedBy(counts[e.engine] ?? 0)}</span>
            </div>
            {e.problem && <div className="banner danger"><Icon name="alert" /><span className="grow">{e.problem}</span></div>}
            {busy ? (
              <div className="field">
                <span className="hint">{t.engines.installing(pct, progress.total > 0 ? `${mb(progress.done)} / ${mb(progress.total)}` : '')}</span>
                <div className="progress" role="progressbar" aria-valuenow={pct} aria-valuemin={0} aria-valuemax={100}><div style={{ width: `${pct}%` }} /></div>
              </div>
            ) : (
              <div className="engine-actions">
                <button type="button" className={`btn${e.installed ? '' : ' primary'}`} disabled={!!progress} onClick={() => install(e.engine)}>
                  <Icon name="download" />{e.installed ? t.engines.reinstall : t.engines.install}
                </button>
                <label className="radio">
                  <input type="radio" name="default-engine" checked={e.default} disabled={!e.installed}
                    onChange={() => useForNew(e.engine)} />
                  {t.engines.useForNew}
                </label>
              </div>
            )}
            {errors[e.engine] && <div className="banner danger" role="alert"><Icon name="alert" /><span className="grow">{errors[e.engine]}</span></div>}
          </div>
        )
      })}
    </div>
  )
}
