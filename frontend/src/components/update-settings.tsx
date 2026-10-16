// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'react'
import { Api, onUpdateProgress } from '../api'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import { useUpdate } from '../hooks/use-update'
import { useConfirm } from './confirm'
import { Icon } from './icons'

/** Settings → Version: the running version, a check button and the installer. */
export function UpdateSettings({ openProfiles }: { openProfiles: number }) {
  const [st, setSt] = useUpdate()
  const [busy, setBusy] = useState<'' | 'check' | 'install'>('')
  const [progress, setProgress] = useState<[number, number] | null>(null)
  const [error, setError] = useState('')
  const [checked, setChecked] = useState(false)
  const confirm = useConfirm()
  useEffect(() => onUpdateProgress((done, total) => setProgress([done, total])), [])
  if (!st) return null

  const check = async () => {
    setBusy('check'); setError('')
    try { setSt(await Api.CheckUpdate()); setChecked(true) } catch (e) { setError(errorText(e)) } finally { setBusy('') }
  }
  const install = async () => {
    if (!(await confirm(t.update.confirm(st.latest)))) return
    setBusy('install'); setError(''); setProgress(null)
    try { await Api.InstallUpdate() } catch (e) { setError(errorText(e)); setBusy('') }
  }
  const auto = async (on: boolean) => {
    try { await Api.SetUpdateAuto(on); setSt({ ...st, auto: on }) } catch (e) { setError(errorText(e)) }
  }
  const pct = progress && progress[1] > 0 ? Math.round((progress[0] / progress[1]) * 100) : null

  return (
    <div className="field">
      <span>{t.update.current(st.current)}</span>
      {st.available ? (
        <div className="banner" role="status">
          <Icon name="download" />
          <span className="grow">
            <b>{t.update.available(st.latest)}</b>
            {st.notes && <span className="updnotes">{st.notes}</span>}
          </span>
        </div>
      ) : checked && <span className="chip ok" style={{ alignSelf: 'flex-start' }}><span className="dot" />{t.update.upToDate}</span>}
      <span className="row">
        {st.available && (
          <button type="button" className="btn sm primary" disabled={busy !== '' || openProfiles > 0} onClick={install}>
            {busy === 'install' ? (pct === null ? t.update.installing : t.update.downloading(pct)) : t.update.install}
          </button>
        )}
        <button type="button" className="btn sm" disabled={busy !== ''} onClick={check}>
          {busy === 'check' ? t.update.checking : t.update.check}
        </button>
      </span>
      {st.available && openProfiles > 0 && <span className="hint">{t.update.closeFirst(openProfiles)}</span>}
      <div className="seg" role="group" aria-label={t.update.autoLabel}>
        <button type="button" aria-pressed={!st.auto} onClick={() => auto(false)}>{t.update.autoOff}</button>
        <button type="button" aria-pressed={st.auto} onClick={() => auto(true)}>{t.update.autoOn}</button>
      </div>
      <span className="hint">{t.update.hint}</span>
      {error && <div className="banner danger" role="alert"><Icon name="alert" /><span className="grow">{error}</span></div>}
    </div>
  )
}
