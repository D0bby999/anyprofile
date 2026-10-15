// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'react'
import { Api, service } from '../api'
import { t } from '../i18n'
import { errorText, nameSeries, parseTags } from '../lib/format'
import { readLastProxy, usableLastProxy, writeLastProxy } from '../lib/last-proxy'
import { guessSite, regionStartUrls, syncStartUrls, type Site } from '../lib/sites'
import { Dialog } from './dialog'
import { CHOOSE_PROXY, freeProxies, ProxySelect, RealIpWarning, SharedProxyWarning } from './proxy-select'
import { SitesPicker } from './site-picker'
import { EngineAdvanced } from './engine-advanced'

type Props = {
  defaultSite: string
  defaults: service.Defaults
  sites: Site[]
  onClose: () => void
  onCreated: () => void
  /** 1 for one profile; more opens the dialog in bulk mode ("Create several
   *  profiles"), the only mode that asks how many. */
  initialCount?: number
}

export function NewProfileDialog({ defaultSite, defaults, sites, onClose, onCreated, initialCount = 1 }: Props) {
  const [name, setName] = useState('')
  const several = initialCount > 1
  const [count, setCount] = useState(initialCount)
  const [platforms, setPlatforms] = useState<string[]>(defaultSite && defaultSite !== 'other' ? [defaultSite] : [])
  // null until the user picks in this dialog: the last profile's choice is offered first.
  const [proxyId, setProxyId] = useState<string | null>(null)
  const [lastProxy] = useState(() => readLastProxy())
  const [bulkProxy, setBulkProxy] = useState<'auto' | 'none'>(defaults.proxy === 'auto' ? 'auto' : 'none')
  const [startUrl, setStartUrl] = useState(() => syncStartUrls('', [], platforms, sites))
  const [tags, setTags] = useState('')
  const [engine, setEngine] = useState('')
  const [proxies, setProxies] = useState<service.ProxyRow[] | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => { Api.ListProxies().then((p) => setProxies(p ?? [])).catch(() => setProxies([])) }, [])
  const chosen = proxyId ?? usableLastProxy(lastProxy, proxies, defaults.proxy === 'auto' ? 'auto' : '')
  // Auto with nothing free must not quietly become "no proxy": ask instead.
  const pick = chosen === 'auto' && proxies !== null && freeProxies(proxies) === 0 ? CHOOSE_PROXY : chosen
  const sharedWith = proxies?.find((p) => p.id === pick)?.usedBy ?? []
  // Platforms that differ by country (TikTok Shop) open the proxy country's
  // page; only a change of country rewrites the lines, picks go through pickSites.
  const country = proxies?.find((p) => p.id === pick)?.country ?? ''
  useEffect(() => setStartUrl((u) => regionStartUrls(u, platforms, sites, country)), [country])

  const pickSites = (ids: string[]) => {
    setStartUrl(syncStartUrls(startUrl, platforms, ids, sites, country))
    setPlatforms(ids)
  }

  // A pasted start page names the platform when none is picked yet.
  const guessFromUrl = () => {
    if (platforms.length) return
    const id = guessSite(startUrl.split('\n')[0], sites)
    if (id) setPlatforms([id])
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
          engine, baseName: name.trim(), count, proxy: bulkProxy, sites: platforms, startUrl: startUrl.trim(), notes: '', tags: parseTags(tags),
        }))
      } else {
        await Api.CreateProfile(service.NewProfile.createFrom({
          engine, name: name.trim(), proxyId: pick, sites: platforms, startUrl: startUrl.trim(), notes: '', tags: parseTags(tags),
        }))
        writeLastProxy(pick)
      }
      onCreated()
    } catch (e) {
      setError(errorText(e))
      setBusy(false)
    }
  }

  const preview = nameSeries(name.trim(), count)

  return (
    <Dialog title={several ? t.palette.createSeveral : t.create.title} icon="plus" onClose={onClose} onSubmit={submit}
      footer={<>
        <button type="button" className="btn ghost" onClick={onClose}>{t.common.cancel}</button>
        <button type="submit" className="btn primary" disabled={!name.trim() || busy || blocked}>{busy ? t.create.creating : t.create.submit}</button>
      </>}>
      <div className={several ? 'grid2' : undefined}>
        <label className="field">
          <span className="label">{bulk ? t.create.namePrefix : t.create.name}</span>
          <input className="input" autoFocus value={name} placeholder={t.create.namePlaceholder} onChange={(e) => setName(e.target.value)} />
        </label>
        {several && (
          <label className="field">
            <span className="label">{t.create.count}</span>
            <input className="input" type="number" min={2} max={100} value={count} onChange={(e) => setCount(Math.max(2, Math.min(100, Number(e.target.value) || 2)))} />
          </label>
        )}
      </div>
      <div className="field">
        <span className="label">{t.create.site}</span>
        <SitesPicker value={platforms} onChange={pickSites} sites={sites} />
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
      {!bulk && sharedWith.length > 0 && <SharedProxyWarning usedBy={sharedWith} />}
      <label className="field">
        <span className="label">{t.create.startUrl}</span>
        <textarea className="textarea" rows={Math.min(5, Math.max(2, startUrl.split('\n').length))} value={startUrl} placeholder={t.create.startUrlHint}
          onChange={(e) => setStartUrl(e.target.value)} onBlur={guessFromUrl} />
      </label>
      <label className="field">
        <span className="label">{t.create.tags}</span>
        <input className="input" value={tags} placeholder={t.create.tagsPlaceholder} onChange={(e) => setTags(e.target.value)} />
      </label>
      <EngineAdvanced value={engine} onChange={setEngine} />
      {bulk && <div className="hint">{t.create.preview} {preview.slice(0, 3).join(', ')}{preview.length > 3 ? ' …' : ''}</div>}
      <span className="hint muted" style={{ fontSize: 'var(--t-xs)' }}>{t.create.identityNote}</span>
      {error && <div className="banner danger" role="alert">{error}</div>}
    </Dialog>
  )
}
