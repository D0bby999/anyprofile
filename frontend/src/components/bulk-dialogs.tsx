// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { Api } from '../api'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import type { Site } from '../lib/sites'
import { Dialog } from './dialog'
import { RealIpWarning } from './proxy-select'
import { SitePicker } from './site-picker'

type BulkProps = { ids: string[]; sites: Site[]; recent: string[]; onClose: () => void; onDone: () => void }

export function BulkSiteDialog({ ids, sites, recent, onClose, onDone }: BulkProps) {
  const [site, setSite] = useState('other')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const apply = async () => {
    setBusy(true)
    setError('')
    try {
      await Api.SetSiteMany(ids, site)
      onDone()
    } catch (e) {
      setError(errorText(e))
      setBusy(false)
    }
  }
  return (
    <Dialog title={t.bulk.siteTitle} icon="pencil" onClose={onClose} onSubmit={apply}
      footer={<>
        <button type="button" className="btn ghost" onClick={onClose}>{t.common.cancel}</button>
        <button type="submit" className="btn primary" disabled={busy}>{t.bulk.apply}</button>
      </>}>
      <SitePicker value={site} onChange={setSite} sites={sites} recent={recent} />
      {error && <div className="banner danger" role="alert">{error}</div>}
    </Dialog>
  )
}

export function BulkTagDialog({ ids, onClose, onDone }: { ids: string[]; onClose: () => void; onDone: () => void }) {
  const [tag, setTag] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const apply = async () => {
    setBusy(true)
    setError('')
    try {
      await Api.AddTagMany(ids, tag.trim())
      onDone()
    } catch (e) {
      setError(errorText(e))
      setBusy(false)
    }
  }
  return (
    <Dialog title={t.bulk.tagTitle} icon="pencil" onClose={onClose} onSubmit={apply}
      footer={<>
        <button type="button" className="btn ghost" onClick={onClose}>{t.common.cancel}</button>
        <button type="submit" className="btn primary" disabled={!tag.trim() || busy}>{t.bulk.apply}</button>
      </>}>
      <input className="input" autoFocus value={tag} onChange={(e) => setTag(e.target.value)} placeholder={t.create.tagsPlaceholder} />
      {error && <div className="banner danger" role="alert">{error}</div>}
    </Dialog>
  )
}

export function BulkProxyDialog({ ids, onClose, onDone }: { ids: string[]; onClose: () => void; onDone: () => void }) {
  const [mode, setMode] = useState<'auto' | 'none'>('auto')
  const [busy, setBusy] = useState(false)
  const [errors, setErrors] = useState<string[]>([])
  const apply = async () => {
    setBusy(true)
    setErrors([])
    const result = await Api.AssignProxiesMany(ids, mode)
    const msgs = Object.entries(result ?? {}).map(([id, msg]) => `${id}: ${errorText(msg)}`)
    if (msgs.length) {
      setErrors(msgs)
      setBusy(false)
    } else {
      onDone()
    }
  }
  return (
    <Dialog title={t.bulk.proxyTitle} icon="proxy" onClose={onClose} onSubmit={apply}
      footer={<>
        <button type="button" className="btn ghost" onClick={onClose}>{t.common.cancel}</button>
        <button type="submit" className="btn primary" disabled={busy}>{t.bulk.apply}</button>
      </>}>
      <div className="seg" role="group" aria-label={t.create.proxy}>
        <button type="button" aria-pressed={mode === 'auto'} onClick={() => setMode('auto')}>{t.bulk.proxyAuto}</button>
        <button type="button" aria-pressed={mode === 'none'} onClick={() => setMode('none')}>{t.bulk.proxyNone}</button>
      </div>
      {mode === 'none' && <RealIpWarning />}
      {errors.map((m) => <div key={m} className="banner danger" role="alert"><span className="grow">{m}</span></div>)}
    </Dialog>
  )
}
