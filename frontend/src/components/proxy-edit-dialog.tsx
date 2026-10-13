// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { Api, service } from '../api'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import { Dialog } from './dialog'

type Props = { row: service.ProxyRow; onClose: () => void; onSaved: () => void }

export function ProxyEditDialog({ row, onClose, onSaved }: Props) {
  const [label, setLabel] = useState(row.label)
  const [note, setNote] = useState(row.note)
  const [expiresOn, setExpiresOn] = useState(row.expiresOn)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const save = async () => {
    if (!label.trim() || busy) return
    setBusy(true)
    setError('')
    try {
      await Api.UpdateProxy(row.id, service.ProxyMeta.createFrom({ label: label.trim(), note, expiresOn }))
      onSaved()
    } catch (e) {
      setError(errorText(e))
      setBusy(false)
    }
  }

  return (
    <Dialog title={t.proxies.editTitle} icon="pencil" onClose={onClose} onSubmit={save}
      footer={<>
        <button type="button" className="btn ghost" onClick={onClose}>{t.common.cancel}</button>
        <button type="submit" className="btn primary" disabled={!label.trim() || busy}>{t.common.save}</button>
      </>}>
      <label className="field">
        <span className="label">{t.proxies.rename}</span>
        <input className="input" autoFocus value={label} onChange={(e) => setLabel(e.target.value)} />
      </label>
      <label className="field">
        <span className="label">{t.proxies.note}</span>
        <textarea className="textarea" rows={3} value={note} onChange={(e) => setNote(e.target.value)} />
      </label>
      <label className="field">
        <span className="label">{t.proxies.expiresOn}</span>
        <input type="date" className="input" value={expiresOn} onChange={(e) => setExpiresOn(e.target.value)} />
      </label>
      {error && <div className="banner danger" role="alert">{error}</div>}
    </Dialog>
  )
}
