// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'react'
import { Api, service, store } from '../api'
import { t } from '../i18n'
import { errorText, parseTags, relativeTime } from '../lib/format'
import { machineLabel } from '../lib/persona'
import type { Site } from '../lib/sites'
import { Dialog } from './dialog'
import { Icon } from './icons'
import { ProxySelect, RealIpWarning } from './proxy-select'
import { useConfirm } from './confirm'
import { SitePicker } from './site-picker'
import { AccountPanel } from './account-panel'
import { HealthPill } from './health-pill'

type Props = { row: service.ProfileRow; sites: Site[]; recent: string[]; onClose: () => void; onSaved: () => void; onTrash: () => void }

export function ProfileEditor({ row, sites, recent, onClose, onSaved, onTrash }: Props) {
  const confirm = useConfirm()
  const [name, setName] = useState(row.name)
  const [site, setSite] = useState(row.site)
  const [login, setLogin] = useState(row.login)
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
        ...row, name: name.trim(), site, proxyId, startUrl: startUrl.trim(), notes, tags: parseTags(tags), cdpNextLaunch: cdp, login,
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
        <span className="label">{t.create.site}</span>
        <SitePicker value={site} onChange={setSite} sites={sites} recent={recent} />
      </div>
      <label className="field">
        <span className="label">{t.create.proxy}</span>
        <ProxySelect value={proxyId} onChange={setProxyId} proxies={proxies} currentProfile={row.name} />
        {proxyId !== row.proxyId && <span className="hint">{t.editor.proxyChangeHint}</span>}
      </label>
      {proxyId === '' && <RealIpWarning />}
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

      <AccountPanel row={row} login={login} onLogin={setLogin} onError={setError} />

      <div className="panel">
        <span className="row"><Icon name="key" className="icon sm" /><b style={{ fontSize: 'var(--t-sm)' }}>{t.editor.identity}</b></span>
        <dl className="facts selectable">
          <dt>{t.editor.persona}</dt><dd>{machineLabel(row)}</dd>
          <dt>{t.editor.engine}</dt><dd>{(t.engines.names[row.engine] ?? row.engine) + ' ' + row.engineVersion}</dd>
          <dt>{t.editor.timezone}</dt><dd>{row.timezone}</dd>
          <dt>{t.editor.languages}</dt><dd>{row.languages}</dd>
          <dt>{t.editor.seed}</dt><dd className="mono">{row.seedHex}</dd>
          <dt>{t.editor.created}</dt><dd>{relativeTime(row.createdAt)}</dd>
          <dt>{t.fp.title}</dt>
          <dd className="row">
            <HealthPill row={row} />
            {row.fpCheckedAt && <span className="muted">{relativeTime(row.fpCheckedAt)}</span>}
            {row.fpVerdict === 'drift' && <span className="muted">{t.fp.diff(row.fpDiff.join(', '))}</span>}
            {row.fpVerdict && (
              <button type="button" className="btn sm ghost"
                onClick={async () => { if (await confirm(t.confirm.resetFingerprint)) Api.ResetFingerprint(row.id).then(() => onSaved()).catch((e) => setError(errorText(e))) }}>
                {t.fp.reset}
              </button>
            )}
          </dd>
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
