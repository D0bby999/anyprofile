// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'react'
import { Api, service, store } from '../api'
import { t } from '../i18n'
import { errorText, parseTags, relativeTime } from '../lib/format'
import { personaLabel } from '../lib/persona'
import { Dialog } from './dialog'
import { Icon } from './icons'
import { ChannelPicker } from './new-profile-dialog'
import { ProxySelect } from './proxy-select'

type Props = { row: service.ProfileRow; onClose: () => void; onSaved: () => void; onTrash: () => void }

export function ProfileEditor({ row, onClose, onSaved, onTrash }: Props) {
  const [name, setName] = useState(row.name)
  const [channelKind, setChannelKind] = useState(row.channelKind)
  const [proxyId, setProxyId] = useState(row.proxyId)
  const [startUrl, setStartUrl] = useState(row.startUrl)
  const [notes, setNotes] = useState(row.notes)
  const [tags, setTags] = useState(row.tags.join(', '))
  const [cdp, setCdp] = useState(row.cdpNextLaunch)
  const [proxies, setProxies] = useState<service.ProxyRow[]>([])
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => { Api.ListProxies().then((p) => setProxies(p ?? [])) }, [])

  const save = async () => {
    if (!name.trim() || saving) return
    setSaving(true)
    setError('')
    try {
      await Api.UpdateProfile(store.Profile.createFrom({
        ...row, name: name.trim(), channelKind, proxyId, startUrl: startUrl.trim(), notes, tags: parseTags(tags), cdpNextLaunch: cdp,
      }))
      onSaved()
    } catch (e) {
      setError(errorText(e))
      setSaving(false)
    }
  }

  const running = row.state !== 'stopped'
  return (
    <Dialog title={t.editor.title} icon="pencil" onClose={onClose} onSubmit={save}
      footer={<>
        <button type="button" className="btn danger" disabled={running} onClick={onTrash}>
          <Icon name="trash" className="icon sm" />{t.profiles.trash}
        </button>
        <span className="spacer" />
        <button type="button" className="btn ghost" onClick={onClose}>{t.common.cancel}</button>
        <button type="submit" className="btn primary" disabled={saving || !name.trim()}>{t.common.save}</button>
      </>}>
      <label className="field">
        <span className="label">{t.create.name}</span>
        <input className="input" value={name} onChange={(e) => setName(e.target.value)} aria-invalid={!name.trim()} />
      </label>
      <div className="field">
        <span className="label">{t.create.channel}</span>
        <ChannelPicker value={channelKind} onChange={setChannelKind} />
      </div>
      <label className="field">
        <span className="label">{t.create.proxy}</span>
        <ProxySelect value={proxyId} onChange={setProxyId} proxies={proxies} currentProfile={row.name} />
        {proxyId !== row.proxyId && <span className="hint">{t.editor.proxyChangeHint}</span>}
      </label>
      <div className="grid2">
        <label className="field">
          <span className="label">{t.create.startUrl}</span>
          <input className="input" value={startUrl} onChange={(e) => setStartUrl(e.target.value)} />
        </label>
        <label className="field">
          <span className="label">{t.create.tags}</span>
          <input className="input" value={tags} placeholder={t.create.tagsPlaceholder} onChange={(e) => setTags(e.target.value)} />
        </label>
      </div>
      <label className="field">
        <span className="label">{t.editor.notes}</span>
        <textarea className="textarea" rows={2} value={notes} onChange={(e) => setNotes(e.target.value)} />
      </label>

      <div className="panel">
        <span className="row"><Icon name="key" className="icon sm" /><b style={{ fontSize: 'var(--t-sm)' }}>{t.editor.identity}</b></span>
        <dl className="facts selectable">
          <dt>{t.editor.persona}</dt><dd>{personaLabel(row.personaId)}</dd>
          <dt>{t.editor.engine}</dt><dd>Fury {row.engineVersion}</dd>
          <dt>{t.editor.timezone}</dt><dd>{row.timezone}</dd>
          <dt>{t.editor.languages}</dt><dd>{row.languages}</dd>
          <dt>{t.editor.seed}</dt><dd className="mono">{row.seedHex}</dd>
          <dt>{t.editor.created}</dt><dd>{relativeTime(row.createdAt)}</dd>
        </dl>
        <span className="hint muted" style={{ fontSize: 'var(--t-xs)' }}>{t.editor.identityHint}</span>
      </div>

      <label className="row" style={{ fontSize: 'var(--t-sm)', fontWeight: 500 }}>
        <input type="checkbox" checked={cdp} onChange={(e) => setCdp(e.target.checked)} />
        {t.editor.cdp}
      </label>
      {cdp && <div className="banner danger"><Icon name="alert" /><span className="grow">{t.editor.cdpWarning}</span></div>}
      {error && <div className="banner danger" role="alert"><Icon name="alert" /><span className="grow">{error}</span></div>}
    </Dialog>
  )
}
