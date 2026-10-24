// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'react'
import { Api, service } from '../api'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import { Dialog } from './dialog'
import { ProxySelect, RealIpWarning } from './proxy-select'

type Props = {
  /** The device to edit; null creates one. */
  row: service.MobileRow | null
  onClose: () => void
  onSaved: (name: string) => void
}

const RAM_CHOICES = [2048, 3072, 4096, 6144]
const CPU_CHOICES = [2, 4, 6, 8]

const splitTags = (s: string) => s.split(',').map((x) => x.trim()).filter(Boolean)

/** Creates a mobile device or edits one. The proxy, memory and cores are
 *  locked while the device runs. */
export function MobileDialog({ row, onClose, onSaved }: Props) {
  const [name, setName] = useState(row?.name ?? '')
  const [proxyId, setProxyId] = useState(row ? row.proxyId : 'auto')
  const [ram, setRam] = useState(row?.ramMb ?? 3072)
  const [cpus, setCpus] = useState(row?.cpus ?? 4)
  const [notes, setNotes] = useState(row?.notes ?? '')
  const [tags, setTags] = useState((row?.tags ?? []).join(', '))
  const [proxies, setProxies] = useState<service.ProxyRow[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const locked = !!row && row.state !== 'stopped'

  useEffect(() => {
    Api.ListProxies().then((p) => {
      setProxies(p ?? [])
      // Without an assignable proxy, "auto" cannot pick one: fall back to choosing.
      if (!row && !(p ?? []).some((x) => x.assignable)) setProxyId('')
    }).catch(() => {})
  }, [row])

  const save = async () => {
    if (!name.trim() || busy) return
    setBusy(true)
    setError('')
    const input = service.MobileInput.createFrom({
      name: name.trim(), proxyId, ramMb: ram, cpus, notes, tags: splitTags(tags), pinned: row?.pinned ?? false,
    })
    try {
      if (row) await Api.UpdateMobileDevice(row.id, input)
      else await Api.CreateMobileDevice(input)
      onSaved(name.trim())
    } catch (e) {
      setError(errorText(e))
      setBusy(false)
    }
  }

  return (
    <Dialog title={row ? t.mobile.dialogEdit : t.mobile.dialogNew} icon="mobile" onClose={onClose} onSubmit={save}
      footer={<>
        <button type="button" className="btn ghost" onClick={onClose} disabled={busy && !row}>{t.common.cancel}</button>
        <button type="submit" className={`btn primary${busy ? ' busy' : ''}`} aria-busy={busy} disabled={!name.trim() || busy}>
          {row ? t.common.save : t.mobile.create}
        </button>
      </>}>
      <label className="field">
        <span className="label">{t.mobile.name}</span>
        <input className="input" autoFocus value={name} placeholder={t.mobile.namePlaceholder} onChange={(e) => setName(e.target.value)} />
      </label>
      <div className="field">
        <span className="label">{t.mobile.proxy}</span>
        {locked ? <span className="muted">{proxies.find((p) => p.id === proxyId)?.label ?? t.common.direct}</span> : (
          <ProxySelect value={proxyId} onChange={setProxyId} proxies={proxies} allowAuto={!row} currentProfile={row?.name} />
        )}
        {proxyId === '' && !locked && <RealIpWarning text={t.mobile.realIpWarning} />}
      </div>
      <div className="row">
        <label className="field grow">
          <span className="label">{t.mobile.ram}</span>
          <select className="select" value={ram} disabled={locked} onChange={(e) => setRam(Number(e.target.value))}>
            {RAM_CHOICES.map((mb) => <option key={mb} value={mb}>{mb / 1024} GB</option>)}
          </select>
        </label>
        <label className="field grow">
          <span className="label">{t.mobile.cpus}</span>
          <select className="select" value={cpus} disabled={locked} onChange={(e) => setCpus(Number(e.target.value))}>
            {CPU_CHOICES.map((n) => <option key={n} value={n}>{n}</option>)}
          </select>
        </label>
      </div>
      {locked && <span className="hint">{t.mobile.closedOnly}</span>}
      <label className="field">
        <span className="label">{t.mobile.notes}</span>
        <textarea className="textarea" rows={2} value={notes} onChange={(e) => setNotes(e.target.value)} />
      </label>
      <label className="field">
        <span className="label">{t.mobile.tags}</span>
        <input className="input" value={tags} onChange={(e) => setTags(e.target.value)} />
        <span className="hint">{t.mobile.tagsHint}</span>
      </label>
      {busy && !row && <div className="banner" role="status">{t.mobile.creating}</div>}
      {error && <div className="banner danger" role="alert">{error}</div>}
    </Dialog>
  )
}
