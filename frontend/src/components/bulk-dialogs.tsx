// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { Api } from '../api'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import type { Site } from '../lib/sites'
import { Dialog } from './dialog'
import { RealIpWarning } from './proxy-select'
import { SitesPicker } from './site-picker'
import { useExtensions } from '../hooks/use-extensions'
import { NoExtensions } from './extension-settings'

type BulkProps = { ids: string[]; sites: Site[]; onClose: () => void; onDone: () => void }

export function BulkSiteDialog({ ids, sites, onClose, onDone }: BulkProps) {
  const [chosen, setChosen] = useState<string[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const apply = async () => {
    setBusy(true)
    setError('')
    try {
      await Api.SetSitesMany(ids, chosen)
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
      <SitesPicker value={chosen} onChange={setChosen} sites={sites} />
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

export function BulkExtensionDialog({ ids, onClose, onDone, onOpenSettings }: { ids: string[]; onClose: () => void; onDone: () => void; onOpenSettings?: () => void }) {
  const { exts } = useExtensions()
  const [ext, setExt] = useState('')
  const [on, setOn] = useState(true)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const chosen = ext || exts[0]?.id || ''
  const apply = async () => {
    if (!chosen) return
    setBusy(true)
    setError('')
    try {
      await Api.SetExtensionMany(ids, chosen, on)
      onDone()
    } catch (e) {
      setError(errorText(e))
      setBusy(false)
    }
  }
  return (
    <Dialog title={t.bulk.extensionTitle} icon="puzzle" onClose={onClose} onSubmit={apply}
      footer={<>
        <button type="button" className="btn ghost" onClick={onClose}>{t.common.cancel}</button>
        <button type="submit" className="btn primary" disabled={busy || !chosen}>{t.bulk.apply}</button>
      </>}>
      {exts.length === 0 ? <NoExtensions onOpenSettings={onOpenSettings && (() => { onClose(); onOpenSettings() })} /> : (
        <>
          <select className="select" value={chosen} onChange={(e) => setExt(e.target.value)} aria-label={t.extensions.title}>
            {exts.map((e) => <option key={e.id} value={e.id}>{e.name} {e.version}</option>)}
          </select>
          <div className="seg" role="group" aria-label={t.bulk.extensionTitle} style={{ alignSelf: 'flex-start' }}>
            <button type="button" aria-pressed={on} onClick={() => setOn(true)}>{t.bulk.extensionOn}</button>
            <button type="button" aria-pressed={!on} onClick={() => setOn(false)}>{t.bulk.extensionOff}</button>
          </div>
          <span className="hint">{t.bulk.extensionHint}</span>
        </>
      )}
      {error && <div className="banner danger" role="alert">{error}</div>}
    </Dialog>
  )
}
