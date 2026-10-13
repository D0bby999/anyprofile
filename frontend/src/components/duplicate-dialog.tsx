// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { Api, service } from '../api'
import { t } from '../i18n'
import { errorText, nameSeries } from '../lib/format'
import { Dialog } from './dialog'

type Props = { row: service.ProfileRow; onClose: () => void; onDone: () => void }

export function DuplicateDialog({ row, onClose, onDone }: Props) {
  const [name, setName] = useState(`${row.name} copy`)
  const [count, setCount] = useState(1)
  const [proxy, setProxy] = useState('auto')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const preview = nameSeries(name, count)

  const submit = async () => {
    if (!name.trim() || busy) return
    setBusy(true)
    setError('')
    try {
      await Api.DuplicateProfile(service.DuplicateInput.createFrom({ sourceId: row.id, baseName: name.trim(), count, proxy }))
      onDone()
    } catch (e) {
      setError(errorText(e))
      setBusy(false)
    }
  }

  return (
    <Dialog title={t.duplicate.title} icon="copy" onClose={onClose} onSubmit={submit}
      footer={<>
        <button type="button" className="btn ghost" onClick={onClose}>{t.common.cancel}</button>
        <button type="submit" className="btn primary" disabled={!name.trim() || busy}>{t.duplicate.submit}</button>
      </>}>
      <label className="field">
        <span className="label">{t.duplicate.name}</span>
        <input className="input" autoFocus value={name} onChange={(e) => setName(e.target.value)} />
      </label>
      <label className="field">
        <span className="label">{t.duplicate.count}</span>
        <input className="input" type="number" min={1} max={50} value={count} onChange={(e) => setCount(Math.max(1, Math.min(50, Number(e.target.value) || 1)))} />
      </label>
      <div className="field">
        <span className="label">{t.create.proxy}</span>
        <div className="seg" role="group" aria-label={t.create.proxy}>
          <button type="button" aria-pressed={proxy === 'auto'} onClick={() => setProxy('auto')}>{t.duplicate.proxyAuto}</button>
          <button type="button" aria-pressed={proxy === 'none'} onClick={() => setProxy('none')}>{t.duplicate.proxyNone}</button>
        </div>
      </div>
      <div className="hint">{t.duplicate.preview} {preview.slice(0, 3).join(', ')}{preview.length > 3 ? ' …' : ''}</div>
      <span className="hint muted" style={{ fontSize: 'var(--t-xs)' }}>{t.duplicate.note}</span>
      {error && <div className="banner danger" role="alert">{error}</div>}
    </Dialog>
  )
}
