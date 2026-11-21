// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'react'
import { Api, onEngineProgress, openExternal, type EngineProgress } from '../api'
import type { useMobile } from '../hooks/use-mobile'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import { InstallProgress } from './install-progress'

export function MobileEngineSettings({ mobile: { avail, reload, rows } }: { mobile: ReturnType<typeof useMobile> }) {
  const copy = t.mobile.engineSettings
  const [busy, setBusy] = useState<'check' | 'install' | ''>('')
  const [progress, setProgress] = useState<EngineProgress | null>(null)
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  useEffect(() => onEngineProgress((p) => {
    if (p.engine === 'anymobile' || p.engine.startsWith('anymobile-')) setProgress(p)
  }), [])

  const act = async (action: 'check' | 'install') => {
    setBusy(action)
    setError('')
    setMessage('')
    try {
      if (action === 'install') {
        await Api.InstallMobileRuntime()
        setMessage(copy.updated)
      } else {
        await Api.ProCheck()
        setMessage(copy.checked)
      }
    } catch (e) {
      setError(errorText(e))
    } finally {
      setBusy('')
      setProgress(null)
      reload()
    }
  }

  if (!avail) return <p className="hint">{copy.loading}</p>
  if (!avail.supported) return <p className="hint">{copy.unsupported}</p>
  if (!avail.install) return <>
    <p className="desc">{copy.external}</p>
    <div className="row"><button type="button" className="btn" onClick={() => openExternal('https://www.ldplayer.net/')}>{t.mobile.ldplayerDownload}</button></div>
  </>

  const engines = avail.engines?.length ? avail.engines : [avail.install]
  const pending = engines.some((st) => st.canUpdate)
  const inUse = rows.some((r) => r.state !== 'stopped')
  const first = !engines.some((st) => st.runtime.installed)
  const installed = (v: string) => v === 'dev' ? copy.development : v || copy.notInstalled
  return <>
    {!avail.pro && <p className="desc">{copy.proRequired}</p>}
    <div className="engines">
      {engines.map(({ runtime, image, canUpdate }) => <div key={runtime.engine} className="engine">
        <div className="engine-head">
          <b>{runtime.engine === 'anymobile-avo' ? 'AnyMobile Metal' : 'AnyMobile (QEMU)'}</b>
          {avail.pro && canUpdate && <span className="chip accent">{copy.available}</span>}
        </div>
        <div className="engine-facts">
          <span>{copy.installed(installed(runtime.installed))}</span>
          <span>{copy.latest(avail.pro ? runtime.latest || copy.notOffered : copy.notOffered)}</span>
          <span>{copy.android(installed(image.installed))}</span>
        </div>
      </div>)}
    </div>
    {avail.pro && <>
      {pending && inUse && <p className="hint">{copy.closeFirst}</p>}
      <div className="row">
        <button type="button" className="btn" onClick={() => act('check')} disabled={!!busy}>{busy === 'check' ? t.update.checking : t.update.check}</button>
        {pending && <button type="button" className="btn primary" onClick={() => act('install')} disabled={!!busy || inUse}>
          {first ? t.mobile.install : t.mobile.engineUpdateBtn}
        </button>}
      </div>
    </>}
    {busy === 'install' && <InstallProgress progress={progress ?? { engine: 'anymobile', done: 0, total: 0 }} />}
    {message && <p className="hint" role="status">{message}</p>}
    {error && <div className="banner danger" role="alert">{error}</div>}
  </>
}
