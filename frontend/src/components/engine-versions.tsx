// SPDX-License-Identifier: AGPL-3.0-or-later
import { Api, service, type EngineProgress } from '../api'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import { useConfirm } from './confirm'
import { Icon } from './icons'
import { InstallProgress } from './install-progress'

type Props = {
  engine: string
  releases: service.EngineRelease[]
  /** This engine's download, if one runs. */
  progress: EngineProgress | null
  /** Some download runs, of any engine: one at a time. */
  locked: boolean
  onInstall: (version: string) => void
  onChanged: () => void
  onError: (msg: string) => void
}

/** An engine's versions: a newer one to download, and every version with Download or Remove. */
export function EngineVersions({ engine, releases, progress, locked, onInstall, onChanged, onError }: Props) {
  const confirm = useConfirm()
  const name = t.engines.names[engine] ?? engine
  const newest = releases[0]
  const newer = newest && !newest.installed && releases.some((r) => r.installed) ? newest : null

  const remove = async (r: service.EngineRelease) => {
    const ok = await confirm({ title: t.engines.removeTitle(name, r.version), body: t.engines.removeBody(r.profiles),
      confirm: t.engines.remove, danger: true })
    if (!ok) return
    try {
      await Api.RemoveEngineVersion(engine, r.version)
    } catch (e) {
      onError(errorText(e))
    } finally {
      onChanged()
    }
  }

  return (
    <>
      {newer && progress?.version !== newer.version && (
        <div className="banner">
          <Icon name="download" />
          <span className="grow"><b>{t.engines.newer(newer.version)}</b> <span className="hint">{t.engines.newerHint}</span></span>
          <button type="button" className="btn sm primary" disabled={locked} onClick={() => onInstall(newer.version)}>{t.engines.download}</button>
        </div>
      )}
      {releases.length > 1 && (
        <details className="engine-versions">
          <summary>{t.engines.versions(releases.length)}</summary>
          <ul>
            {releases.map((r) => (
              <li key={r.version}>
                <span className="ver">{r.version}</span>
                <span className="muted">{t.engines.chromium(r.chromium)}</span>
                {r.date && <span className="muted">{r.date}</span>}
                {r.problem
                  ? <span className="chip danger">{t.engines.damaged}</span>
                  : r.installed
                    ? <span className="chip ok">{t.engines.installedShort}</span>
                    : <span className="chip">{t.engines.notDownloaded}</span>}
                {r.profiles > 0 && <span className="muted">{t.engines.profilesOn(r.profiles)}</span>}
                <span className="grow" />
                {progress?.version === r.version
                  ? <InstallProgress progress={progress} />
                  : r.installed
                    ? <button type="button" className="btn sm ghost" disabled={locked} onClick={() => remove(r)}>{t.engines.remove}</button>
                    : <button type="button" className="btn sm" disabled={locked} onClick={() => onInstall(r.version)}>{t.engines.download}</button>}
              </li>
            ))}
          </ul>
        </details>
      )}
    </>
  )
}
