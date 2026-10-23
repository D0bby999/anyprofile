// SPDX-License-Identifier: AGPL-3.0-or-later
import { service } from '../api'
import { t } from '../i18n'
import { Icon } from './icons'
import { BuyProxiesButton } from './partner-grid'

type Props = { value: string; onChange: (v: string) => void; proxies: service.ProxyRow[]; allowAuto?: boolean; currentProfile?: string }

export const freeProxies = (proxies: service.ProxyRow[]) => proxies.filter((p) => !p.rotating && p.usedBy.length === 0).length

/** Proxy picker. Rotating proxies are listed but disabled: they cannot hold an identity.
 *  Labels never carry the proxy username: they are host:port or the user's own name. */
export function ProxySelect({ value, onChange, proxies, allowAuto, currentProfile }: Props) {
  const free = freeProxies(proxies)
  return (
    <select className="select" value={value} onChange={(e) => onChange(e.target.value)} aria-label={t.create.proxy}>
      {allowAuto && <option value="auto" disabled={free === 0}>{free ? t.create.proxyAutoFree(free) : t.create.proxyAutoEmpty}</option>}
      <option value="">{t.create.proxyDirect}</option>
      {proxies.map((p) => {
        const others = p.usedBy.filter((n) => n !== currentProfile)
        const where = [p.country, p.exitIp].filter(Boolean).join(' ')
        const note = p.rotating ? t.proxies.rotating : others.length ? others.join(', ') : ''
        return (
          <option key={p.id} value={p.id} disabled={p.rotating}>
            {p.label}{where ? ` · ${where}` : ''}{note ? ` (${note})` : ''}
          </option>
        )
      })}
    </select>
  )
}

/** Said wherever a choice leads to a profile or device without proxy, with the way to get one. */
export function RealIpWarning({ text = t.create.realIpWarning }: { text?: string }) {
  return (
    <div className="realip" role="note">
      <Icon name="alert" className="icon sm" /><span className="grow">{text}</span>
      <BuyProxiesButton className="btn sm buy" />
    </div>
  )
}
