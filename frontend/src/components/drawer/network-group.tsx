// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'react'
import { service } from '../../api'
import { t } from '../../i18n'
import { ProxySelect, RealIpWarning } from '../proxy-select'
import { Icon } from '../icons'
import { flagText } from '../../lib/flags'

type Props = {
  row: service.ProfileRow
  proxies: service.ProxyRow[]
  running: boolean
  onApply: (proxyId: string) => Promise<boolean>
  onMatchIp: () => void
}

/** Proxy and what it gives the profile. A new proxy is saved only on Apply. */
export function NetworkGroup({ row, proxies, running, onApply, onMatchIp }: Props) {
  const [proxyId, setProxyId] = useState(row.proxyId)
  const [busy, setBusy] = useState(false)
  useEffect(() => setProxyId(row.proxyId), [row.proxyId])
  const changed = proxyId !== row.proxyId
  const facts = [row.proxyId ? row.proxyCountry : '', row.proxyId ? row.proxyExitIp : t.common.direct, row.timezone, row.languages]
    .filter(Boolean).join(' · ')

  return (
    <section className="dgroup" aria-label={t.drawer.network}>
      <h3>{t.drawer.network}</h3>
      <div className="row nowrap-row">
        <ProxySelect value={proxyId} onChange={setProxyId} proxies={proxies} currentProfile={row.name} />
        {changed && (
          <button type="button" className={`btn sm primary${busy ? ' busy' : ''}`} aria-busy={busy} disabled={busy}
            onClick={async () => { setBusy(true); if (!(await onApply(proxyId))) setProxyId(row.proxyId); setBusy(false) }}>
            {t.drawer.apply}
          </button>
        )}
      </div>
      <span className="dfacts">{facts}</span>
      {changed && <span className="hint">{t.editor.proxyChangeHint}</span>}
      {changed && running && <span className="hint">{t.drawer.nextTime}</span>}
      {proxyId === '' && <RealIpWarning />}
      {(row.flags ?? []).filter((f) => f.level !== 'info').map((f) => (
        <div key={f.id} className={`banner ${f.level === 'danger' ? 'danger' : 'warn'}`}>
          <Icon name="alert" /><span className="grow">{flagText(f).text}</span>
          {f.id === 'tz_mismatch' && <button type="button" className="btn sm" onClick={onMatchIp}>{t.sessions.matchIp}</button>}
        </div>
      ))}
    </section>
  )
}
