// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { Api, service } from '../api'
import { t } from '../i18n'
import { errorText, formatBytes } from '../lib/format'
import { Icon } from './icons'

const MIN_PASSWORD = 8

/** Backup of everything to one password-protected file, and its restore. */
export function BackupSettings() {
  return (
    <>
      <p className="desc" style={{ margin: 0 }}>{t.backup.text}</p>
      <BackupExport />
      <BackupRestore titled />
    </>
  )
}

function BackupExport() {
  const [pw, setPw] = useState('')
  const [again, setAgain] = useState('')
  const [busy, setBusy] = useState(false)
  const [done, setDone] = useState<service.BackupResult | null>(null)
  const [error, setError] = useState('')
  const short = pw.length > 0 && pw.length < MIN_PASSWORD
  const mismatch = again.length > 0 && again !== pw
  const ready = pw.length >= MIN_PASSWORD && again === pw && !busy

  const save = async () => {
    if (!ready) return
    setBusy(true)
    setError('')
    setDone(null)
    try {
      const res = await Api.ExportBackup(pw)
      if (res.bytes > 0) {
        setDone(res)
        setPw('')
        setAgain('')
      }
    } catch (e) {
      setError(errorText(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <form className="field" onSubmit={(e) => { e.preventDefault(); save() }}>
      <span className="label">{t.backup.saveTitle}</span>
      <div className="grid2">
        <label className="field">
          <span className="label">{t.backup.password}</span>
          <input className="input" type="password" autoComplete="new-password" value={pw} onChange={(e) => setPw(e.target.value)} />
        </label>
        <label className="field">
          <span className="label">{t.backup.again}</span>
          <input className="input" type="password" autoComplete="new-password" value={again} onChange={(e) => setAgain(e.target.value)} />
        </label>
      </div>
      <span className="hint" role={short || mismatch ? 'alert' : undefined}>{mismatch ? t.backup.mismatch : t.backup.passwordHint(MIN_PASSWORD)}</span>
      <div className="row">
        <button type="submit" className={`btn primary${busy ? ' busy' : ''}`} aria-busy={busy} disabled={!ready}>
          <Icon name={busy ? 'refresh' : 'download'} />{busy ? t.backup.saving : t.backup.save}
        </button>
      </div>
      {done && (
        <div className="banner ok" role="status"><Icon name="check" />
          <span className="grow">{t.backup.saved(done.profiles, done.proxies, formatBytes(done.bytes))}</span></div>
      )}
      {error && <div className="banner danger" role="alert"><Icon name="alert" /><span className="grow">{error}</span></div>}
    </form>
  )
}

/** Restore: pick the file, type its password. Also offered on the first-run page. */
export function BackupRestore({ titled, onRestored }: { titled?: boolean; onRestored?: (res: service.BackupResult) => void }) {
  const [file, setFile] = useState('')
  const [pw, setPw] = useState('')
  const [busy, setBusy] = useState(false)
  const [done, setDone] = useState<service.BackupResult | null>(null)
  const [error, setError] = useState('')

  const pick = async () => {
    setError('')
    setDone(null)
    try {
      const name = await Api.PickBackup()
      if (name) setFile(name)
    } catch (e) {
      setError(errorText(e))
    }
  }

  const restore = async () => {
    if (!file || !pw || busy) return
    setBusy(true)
    setError('')
    try {
      const res = await Api.ImportBackup(pw)
      setDone(res)
      setFile('')
      setPw('')
      onRestored?.(res)
    } catch (e) {
      setError(errorText(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <form className="field" onSubmit={(e) => { e.preventDefault(); restore() }}>
      {titled && <span className="label">{t.backup.restoreTitle}</span>}
      <div className="row">
        <button type="button" className="btn" onClick={pick} disabled={busy}><Icon name="restore" />{t.backup.pick}</button>
        {file && <span className="mono selectable">{file}</span>}
      </div>
      {file && (
        <>
          <label className="field">
            <span className="label">{t.backup.passwordOf}</span>
            <input className="input" type="password" autoComplete="off" autoFocus value={pw} onChange={(e) => setPw(e.target.value)} />
          </label>
          <div className="row">
            <button type="submit" className={`btn primary${busy ? ' busy' : ''}`} aria-busy={busy} disabled={!pw || busy}>
              <Icon name={busy ? 'refresh' : 'check'} />{busy ? t.backup.restoring : t.backup.restore}
            </button>
          </div>
          <span className="hint">{t.backup.restoreHint}</span>
        </>
      )}
      {done && (
        <div className="banner ok" role="status"><Icon name="check" />
          <span className="grow">{t.backup.restored(done.profiles, done.proxies)}
            {done.profilesHere + done.proxiesHere > 0 && <> {t.backup.alreadyHere(done.profilesHere, done.proxiesHere)}</>}</span></div>
      )}
      {error && <div className="banner danger" role="alert"><Icon name="alert" /><span className="grow">{error}</span></div>}
    </form>
  )
}
