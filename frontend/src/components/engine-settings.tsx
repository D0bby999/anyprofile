// SPDX-License-Identifier: AGPL-3.0-or-later
import { useCallback, useEffect, useState } from 'react'
import { Api, main, onDataChanged, onEngineProgress, service, type EngineProgress } from '../api'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import { EngineVersions } from './engine-versions'
import { Icon } from './icons'
import { InstallProgress } from './install-progress'
import { useToast } from './toast'

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
  const toast = useToast()
  const [progress, setProgress] = useState<EngineProgress | null>(null)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [releases, setReleases] = useState<Record<string, service.EngineRelease[]>>({})
  useEffect(() => onEngineProgress(setProgress), [])

  const ids = engines.map((e) => e.engine).join(',')
  const loadReleases = useCallback(() => {
    for (const id of ids.split(',').filter(Boolean)) {
      Api.EngineReleases(id).then((rs) => setReleases((x) => ({ ...x, [id]: rs ?? [] }))).catch(() => {})
    }
  }, [ids])
  useEffect(() => { loadReleases() }, [loadReleases])
  useEffect(() => onDataChanged(['engines', 'profiles'], loadReleases), [loadReleases])
  const changed = () => { loadReleases(); onChanged() }

  /** Installs a version, or the engine's newest without one. */
  const install = async (id: string, version?: string) => {
    setErrors((e) => ({ ...e, [id]: '' }))
    setProgress({ engine: id, version, done: 0, total: -1 })
    try {
      await (version ? Api.InstallEngineVersion(id, version) : Api.InstallEngine(id))
    } catch (e) {
      setErrors((x) => ({ ...x, [id]: errorText(e) }))
    } finally {
      setProgress(null)
      changed()
    }
  }

  const useForNew = async (id: string) => {
    setErrors((e) => ({ ...e, [id]: '' }))
    try {
      await Api.SetDefaults(service.Defaults.createFrom({ ...defaults, engine: id }))
      // New profiles only: say which existing ones this does not move, so the
      // next Open is not a surprise.
      const keep = engines
        .filter((x) => x.engine !== id && (counts[x.engine] ?? 0) > 0)
        .map((x) => `${t.engines.names[x.engine] ?? x.engine} (${counts[x.engine]})`)
        .join(', ')
      toast.success(keep ? t.engines.defaultSavedOthers(engineName(id), keep) : t.engines.defaultSaved(engineName(id)))
      onChanged()
    } catch (e) {
      setErrors((x) => ({ ...x, [id]: errorText(e) }))
    }
  }

  return (
    <div className="engines">
      <p className="hint">{t.engines.defaultHint}</p>
      {engines.map((e) => {
        // A version's own download shows in its row; the engine's newest shows here.
        const busy = progress?.engine === e.engine && !progress.version
        // Keep the optional Apostate choice visible; unused older engines stay folded away.
        const older = e.engine !== 'apostate' && !e.default && !e.installed && !(counts[e.engine] ?? 0) && !busy
        const block = (
          <div key={e.engine} className="engine" aria-busy={busy}>
            <div className="engine-head">
              <b>{engineName(e.engine)}</b>
              {e.engine === 'anybrowser' && <span className="chip accent">{t.engines.recommended}</span>}
              <span className="grow" />
              {e.problem
                ? <span className="chip danger"><span className="dot" />{t.engines.notInstalled}</span>
                : e.installed
                  ? <span className="chip ok"><span className="dot" />{t.engines.installed}</span>
                  : <span className="chip">{t.engines.notInstalled}</span>}
            </div>
            <p className="desc">{e.engine === 'anybrowser' ? t.engines.anybrowserDesc
              : e.engine === 'apostate' ? t.engines.apostateDesc : t.engines.furyDesc}</p>
            <div className="engine-facts">
              <span>{t.engines.version(e.version)}</span>
              <span>{t.engines.chromium(e.chromium)}</span>
              <span>{t.engines.usedBy(counts[e.engine] ?? 0)}</span>
            </div>
            {e.problem && <div className="banner danger"><Icon name="alert" /><span className="grow">{e.problem}</span></div>}
            <EngineVersions engine={e.engine} releases={releases[e.engine] ?? []}
              progress={progress?.engine === e.engine ? progress : null} locked={!!progress}
              onInstall={(v) => install(e.engine, v)} onChanged={changed}
              onError={(msg) => setErrors((x) => ({ ...x, [e.engine]: msg }))} />
            {busy ? (
              <InstallProgress progress={progress} />
            ) : (
              <div className="engine-actions">
                <button type="button" className={`btn${e.installed || !e.default ? '' : ' primary'}`} disabled={!!progress} onClick={() => install(e.engine)}>
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
        return older ? (
          <details key={e.engine} className="engine-older" open={!!errors[e.engine]}>
            <summary>{t.engines.older}: {engineName(e.engine)}</summary>
            {block}
          </details>
        ) : block
      })}
    </div>
  )
}
