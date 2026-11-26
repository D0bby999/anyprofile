// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { Api, service } from '../api'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import { Dialog } from './dialog'

type Props = {
  source: service.MobileRow
  onClose: () => void
  onCloned: (count: number) => void
}

/** APFS instant copy-on-write cloner for a stopped Golden Master device. */
export function MobileCloneDialog({ source, onClose, onCloned }: Props) {
  const [name, setName] = useState(`${source.name} Copy`)
  const [count, setCount] = useState(1)
  const [resetGoogle, setResetGoogle] = useState(true)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setErr('')
    setBusy(true)
    try {
      const list = await Api.CloneMobileDevice(source.id, name.trim(), count, resetGoogle)
      onCloned(list?.length || count)
      onClose()
    } catch (ex) {
      setErr(errorText(ex))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog icon="copy" title={t.mobile.cloneTitle(source.name)} onClose={onClose}
      footer={<>
        <button type="button" className="btn" disabled={busy} onClick={onClose}>{t.common.cancel}</button>
        <button type="submit" form="mobile-clone-form" className="btn primary" disabled={busy || !name.trim()}>{t.mobile.cloneRun(count)}</button>
      </>}>
      <form id="mobile-clone-form" onSubmit={submit} className="stack">
        <p className="desc">{t.mobile.cloneHint}</p>
        <label className="field">
          <span>{t.mobile.cloneName}</span>
          <input className="input" value={name} onChange={(e) => setName(e.target.value)} required autoFocus />
        </label>
        <label className="field">
          <span>{t.mobile.cloneCount}</span>
          <div className="row" style={{ gap: 6 }}>
            <input className="input" type="number" min={1} max={10} value={count}
              onChange={(e) => setCount(Math.max(1, Math.min(10, Number(e.target.value) || 1)))}
              style={{ width: 84 }} />
            {[1, 2, 3, 5].map((n) => (
              <button key={n} type="button" className={`btn sm${count === n ? ' primary' : ''}`}
                onClick={() => setCount(n)}>{n}</button>
            ))}
          </div>
        </label>
        <label className="row checkrow">
          <input type="checkbox" checked={resetGoogle} onChange={(e) => setResetGoogle(e.target.checked)} />
          <span>{t.mobile.cloneResetGoogle}</span>
        </label>
        {err && <div className="banner danger" role="alert">{err}</div>}
      </form>
    </Dialog>
  )
}
