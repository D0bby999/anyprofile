// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { Api, service } from '../api'
import { t } from '../i18n'
import { errorText } from '../lib/format'
import { Dialog } from './dialog'
import { Icon } from './icons'

type Props = { onClose: () => void; onAdded: (res: service.AddProxyResult) => void }
type Scheme = 'http' | 'https' | 'socks5'

export function ProxyAddDialog({ onClose, onAdded }: Props) {
  const [label, setLabel] = useState('')
  const [scheme, setScheme] = useState<Scheme>('http')
  const [host, setHost] = useState('')
  const [port, setPort] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [expiresOn, setExpiresOn] = useState('')
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const portNum = Number.parseInt(port, 10)
  const valid = !!host.trim() && Number.isInteger(portNum) && portNum >= 1 && portNum <= 65535

  const submit = async () => {
    if (!valid || busy) return
    setBusy(true)
    setError('')
    try {
      const res = await Api.AddProxy(service.ProxyInput.createFrom({
        label: label.trim(),
        scheme,
        host: host.trim(),
        port: portNum,
        username: username.trim(),
        password,
        note,
        expiresOn,
      }))
      onAdded(res)
    } catch (e) {
      setError(errorText(e))
      setBusy(false)
    }
  }

  return (
    <Dialog title={t.proxies.newTitle} icon="proxy" onClose={busy ? () => {} : onClose} onSubmit={submit}
      footer={<>
        <button type="button" className="btn ghost" disabled={busy} onClick={onClose}>{t.common.cancel}</button>
        <button type="submit" className={`btn primary${busy ? ' busy' : ''}`} aria-busy={busy} disabled={!valid || busy}>
          <Icon name={busy ? 'refresh' : 'plus'} />{busy ? t.proxies.addSubmitting : t.proxies.addSubmit}
        </button>
      </>}>
      <div className="grid2">
        <label className="field">
          <span className="label">{t.proxies.label}</span>
          <input className="input" disabled={busy} value={label} placeholder={t.proxies.labelPlaceholder} onChange={(e) => setLabel(e.target.value)} />
        </label>
        <label className="field">
          <span className="label">{t.proxies.scheme}</span>
          <select className="select" disabled={busy} value={scheme} onChange={(e) => setScheme(e.target.value as Scheme)}>
            <option value="http">HTTP</option>
            <option value="https">HTTPS</option>
            <option value="socks5">SOCKS5</option>
          </select>
        </label>
      </div>
      <div className="row" style={{ alignItems: 'flex-start' }}>
        <label className="field" style={{ flex: 3 }}>
          <span className="label">{t.proxies.host}</span>
          <input className="input mono" autoFocus required disabled={busy} value={host} placeholder={t.proxies.hostPlaceholder}
            spellCheck={false} autoComplete="off" onChange={(e) => setHost(e.target.value)} />
        </label>
        <label className="field" style={{ flex: 1, minWidth: 96 }}>
          <span className="label">{t.proxies.port}</span>
          <input className="input mono" type="number" min={1} max={65535} required disabled={busy} value={port} placeholder="8080"
            onChange={(e) => setPort(e.target.value)} />
        </label>
      </div>
      <div className="grid2">
        <label className="field">
          <span className="label">{t.proxies.username}</span>
          <input className="input" disabled={busy} value={username} autoComplete="off" spellCheck={false} onChange={(e) => setUsername(e.target.value)} />
        </label>
        <label className="field">
          <span className="label">{t.proxies.password}</span>
          <input className="input" type="password" disabled={busy} value={password} autoComplete="new-password" onChange={(e) => setPassword(e.target.value)} />
        </label>
      </div>
      <div className="grid2">
        <label className="field">
          <span className="label">{t.proxies.expiresOn}</span>
          <input type="date" className="input" disabled={busy} value={expiresOn} onChange={(e) => setExpiresOn(e.target.value)} />
        </label>
        <label className="field">
          <span className="label">{t.proxies.note}</span>
          <input className="input" disabled={busy} value={note} placeholder={t.proxies.notePlaceholder} onChange={(e) => setNote(e.target.value)} />
        </label>
      </div>
      <span className="row hint"><Icon name="key" className="icon sm" />{t.proxies.passwordNote}</span>
      {error && <div className="banner danger" role="alert"><Icon name="alert" /><span>{error}</span></div>}
    </Dialog>
  )
}

