// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'react'
import { Api, service } from '../api'
import { t } from '../i18n'
import { errorText, nameSeries, parseTags } from '../lib/format'
import { guessSite, siteById, type Site } from '../lib/sites'
import { Dialog } from './dialog'
import { CHOOSE_PROXY, freeProxies, ProxySelect, RealIpWarning } from './proxy-select'
import { SitePicker } from './site-picker'
import { EngineAdvanced } from './engine-advanced'

type Props = {
  defaultSite: string
  defaults: service.Defaults
  sites: Site[]
  recent: string[]
  onClose: () => void
  onCreated: () => void
  /** 1 for one profile; more opens the dialog in bulk mode. */
  initialCount?: number
}

export function NewProfileDialog({ defaultSite, defaults, sites, recent, onClose, onCreated, initialCount = 1 }: Props) {
  const [name, setName] = useState('')
  const [count, setCount] = useState(initialCount)
  const [site, setSite] = useState(defaultSite)
  const [proxyId, setProxyId] = useState(defaults.proxy === 'auto' ? 'auto' : '')
  const [bulkProxy, setBulkProxy] = useState<'auto' | 'none'>(defaults.proxy === 'auto' ? 'auto' : 'none')
  const [startUrl, setStartUrl] = useState(siteById(sites, defaultSite).startUrl)
  const [urlTouched, setUrlTouched] = useState(false)
  const [tags, setTags] = useState('')
  const [engine, setEngine] = useState('')
  const [proxies, setProxies] = useState<service.ProxyRow[] | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => { Api.ListProxies().then((p) => setProxies(p ?? [])).catch(() => setProxies([])) }, [])
  // Auto with nothing free must not quietly become "no proxy": ask instead.
  const pick = proxyId === 'auto' && proxies !== null && freeProxies(proxies) === 0 ? CHOOSE_PROXY : proxyId

  const pickSite = (id: string) => {
    setSite(id)
    if (!urlTouched) setStartUrl(siteById(sites, id).startUrl)
  }

  const guessFromUrl = () => {
    if (site !== 'other') return
    const id = guessSite(startUrl.split('\n')[0], sites)
    if (id) setSite(id)
  }

  const bulk = count > 1
  const blocked = !bulk && pick === CHOOSE_PROXY
  const direct = bulk ? bulkProxy === 'none' : pick === ''

  const submit = async () => {
    if (!name.trim() || busy || blocked) return
    setBusy(true)
    setError('')
    try {
      if (bulk) {
        await Api.CreateProfiles(service.BulkCreate.createFrom({
          engine, baseName: name.trim(), count, proxy: bulkProxy, site, startUrl: startUrl.trim(), notes: '', tags: parseTags(tags),
        }))
      } else {
        await Api.CreateProfile(service.NewProfile.createFrom({
          engine, name: name.trim(), proxyId: pick, site, startUrl: startUrl.trim(), notes: '', tags: parseTags(tags),
        }))
      }
      onCreated()
    } catch (e) {
      setError(errorText(e))
      setBusy(false)
    }
  }

  const preview = nameSeries(name.trim(), count)

  return (
    <Dialog title={t.create.title} icon="plus" onClose={onClose} onSubmit={submit}
      footer={<>
        <button type="button" className="btn ghost" onClick={onClose}>{t.common.cancel}</button>
        <button type="submit" className="btn primary" disabled={!name.trim() || busy || blocked}>{busy ? t.create.creating : t.create.submit}</button>
      </>}>
      <div className="grid2">
        <label className="field">
          <span className="label">{bulk ? t.create.namePrefix : t.create.name}</span>
          <input className="input" autoFocus value={name} placeholder={t.create.namePlaceholder} onChange={(e) => setName(e.target.value)} />
        </label>
        <label className="field">
          <span className="label">{t.create.count}</span>
          <input className="input" type="number" min={1} max={100} value={count} onChange={(e) => setCount(Math.max(1, Math.min(100, Number(e.target.value) || 1)))} />
        </label>
      </div>
      <div className="field">
        <span className="label">{t.create.site}</span>
        <SitePicker value={site} onChange={pickSite} sites={sites} recent={recent} />
      </div>
      {bulk ? (
        <div className="field">
          <span className="label">{t.create.proxy}</span>
          <div className="seg" role="group" aria-label={t.create.proxy}>
            <button type="button" aria-pressed={bulkProxy === 'auto'} onClick={() => setBulkProxy('auto')}>{t.duplicate.proxyAuto}</button>
            <button type="button" aria-pressed={bulkProxy === 'none'} onClick={() => setBulkProxy('none')}>{t.duplicate.proxyNone}</button>
          </div>
        </div>
      ) : (
        <label className="field">
          <span className="label">{t.create.proxy}</span>
          <ProxySelect value={pick} onChange={setProxyId} proxies={proxies ?? []} allowAuto />
          {!direct && <span className="hint">{t.create.proxyHint}</span>}
        </label>
      )}
      {direct && <RealIpWarning />}
      <div className="grid2">
        <label className="field">
          <span className="label">{t.create.startUrl}</span>
          <textarea className="textarea" rows={2} value={startUrl} placeholder={t.create.startUrlHint}
            onChange={(e) => { setStartUrl(e.target.value); setUrlTouched(true) }} onBlur={guessFromUrl} />
        </label>
        <label className="field">
          <span className="label">{t.create.tags}</span>
          <input className="input" value={tags} placeholder={t.create.tagsPlaceholder} onChange={(e) => setTags(e.target.value)} />
        </label>
      </div>
      <EngineAdvanced value={engine} onChange={setEngine} />
      {bulk && <div className="hint">{t.create.preview} {preview.slice(0, 3).join(', ')}{preview.length > 3 ? ' …' : ''}</div>}
      <span className="hint muted" style={{ fontSize: 'var(--t-xs)' }}>{t.create.identityNote}</span>
      {error && <div className="banner danger" role="alert">{error}</div>}
    </Dialog>
  )
}
