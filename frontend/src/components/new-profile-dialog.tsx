// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'react'
import { Api, service } from '../api'
import { t } from '../i18n'
import { errorText, parseTags } from '../lib/format'
import { Dialog } from './dialog'
import { ChannelGlyph } from './icons'
import { ProxySelect } from './proxy-select'

type Props = { defaultChannel: string; onClose: () => void; onCreated: () => void }

export function ChannelPicker({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  return (
    <div className="seg" role="group" aria-label={t.create.channel}>
      {(['tiktok', 'youtube', 'other'] as const).map((k) => (
        <button key={k} type="button" aria-pressed={value === k} onClick={() => onChange(k)}>
          <ChannelGlyph kind={k} />{t.channel[k]}
        </button>
      ))}
    </div>
  )
}

export function NewProfileDialog({ defaultChannel, onClose, onCreated }: Props) {
  const [name, setName] = useState('')
  const [channel, setChannel] = useState(defaultChannel)
  const [proxyId, setProxyId] = useState('auto')
  const [startUrl, setStartUrl] = useState(t.channelStartUrl[defaultChannel] ?? '')
  const [urlTouched, setUrlTouched] = useState(false)
  const [tags, setTags] = useState('')
  const [proxies, setProxies] = useState<service.ProxyRow[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => { Api.ListProxies().then((p) => setProxies(p ?? [])) }, [])

  const pickChannel = (k: string) => {
    setChannel(k)
    if (!urlTouched) setStartUrl(t.channelStartUrl[k] ?? '')
  }

  const submit = async () => {
    if (!name.trim() || busy) return
    setBusy(true)
    setError('')
    try {
      await Api.CreateProfile(service.NewProfile.createFrom({
        name: name.trim(), proxyId, channelKind: channel, startUrl: startUrl.trim(), notes: '', tags: parseTags(tags),
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
        <span className="label">{t.create.channel}</span>
        <ChannelPicker value={channel} onChange={pickChannel} />
      </div>
      <label className="field">
        <span className="label">{t.create.proxy}</span>
        <ProxySelect value={proxyId} onChange={setProxyId} proxies={proxies} allowAuto />
        <span className="hint">{t.create.proxyHint}</span>
      </label>
      <div className="grid2">
        <label className="field">
          <span className="label">{t.create.startUrl}</span>
          <input className="input" value={startUrl} onChange={(e) => { setStartUrl(e.target.value); setUrlTouched(true) }} />
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
