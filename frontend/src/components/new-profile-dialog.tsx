// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'react'
import { Api, service } from '../api'
import { t } from '../i18n'
import { errorText, parseTags } from '../lib/format'
import { guessSite, siteById, type Site } from '../lib/sites'
import { Dialog } from './dialog'
import { ProxySelect } from './proxy-select'
import { SitePicker } from './site-picker'

type Props = {
  defaultSite: string
  sites: Site[]
  recent: string[]
  onClose: () => void
  onCreated: () => void
}

export function NewProfileDialog({ defaultSite, sites, recent, onClose, onCreated }: Props) {
  const [name, setName] = useState('')
  const [site, setSite] = useState(defaultSite)
  const [proxyId, setProxyId] = useState('auto')
  const [startUrl, setStartUrl] = useState(siteById(sites, defaultSite).startUrl)
  const [urlTouched, setUrlTouched] = useState(false)
  const [tags, setTags] = useState('')
  const [proxies, setProxies] = useState<service.ProxyRow[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => { Api.ListProxies().then((p) => setProxies(p ?? [])) }, [])

  const pickSite = (id: string) => {
    setSite(id)
    if (!urlTouched) setStartUrl(siteById(sites, id).startUrl)
  }

  // Pasting a known platform's URL into an "Other" profile picks that platform.
  const guessFromUrl = () => {
    if (site !== 'other') return
    const id = guessSite(startUrl, sites)
    if (id) setSite(id)
  }

  const submit = async () => {
    if (!name.trim() || busy) return
    setBusy(true)
    setError('')
    try {
      await Api.CreateProfile(service.NewProfile.createFrom({
        name: name.trim(), proxyId, site, startUrl: startUrl.trim(), notes: '', tags: parseTags(tags),
      }))
      onCreated()
    } catch (e) {
      setError(errorText(e))
      setBusy(false)
    }
  }

  return (
    <Dialog title={t.create.title} icon="plus" onClose={onClose} onSubmit={submit}
      footer={<>
        <button type="button" className="btn ghost" onClick={onClose}>{t.common.cancel}</button>
        <button type="submit" className="btn primary" disabled={!name.trim() || busy}>{busy ? t.create.creating : t.create.submit}</button>
      </>}>
      <label className="field">
        <span className="label">{t.create.name}</span>
        <input className="input" autoFocus value={name} placeholder={t.create.namePlaceholder} onChange={(e) => setName(e.target.value)} />
      </label>
      <div className="field">
        <span className="label">{t.create.site}</span>
        <SitePicker value={site} onChange={pickSite} sites={sites} recent={recent} />
      </div>
      <label className="field">
        <span className="label">{t.create.proxy}</span>
        <ProxySelect value={proxyId} onChange={setProxyId} proxies={proxies} allowAuto />
        <span className="hint">{t.create.proxyHint}</span>
      </label>
      <div className="grid2">
        <label className="field">
          <span className="label">{t.create.startUrl}</span>
          <input className="input" value={startUrl} onChange={(e) => { setStartUrl(e.target.value); setUrlTouched(true) }} onBlur={guessFromUrl} />
        </label>
        <label className="field">
          <span className="label">{t.create.tags}</span>
          <input className="input" value={tags} placeholder={t.create.tagsPlaceholder} onChange={(e) => setTags(e.target.value)} />
        </label>
      </div>
      <span className="hint muted" style={{ fontSize: 'var(--t-xs)' }}>{t.create.identityNote}</span>
      {error && <div className="banner danger" role="alert">{error}</div>}
    </Dialog>
  )
}
