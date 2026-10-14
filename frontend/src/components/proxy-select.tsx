// SPDX-License-Identifier: AGPL-3.0-or-later
import { service } from '../api'
import { t } from '../i18n'
import { Icon } from './icons'

/** Value of a picker that has nothing chosen yet (Auto was the default, but no proxy is free). */
export const CHOOSE_PROXY = '__choose'

type Props = { value: string; onChange: (v: string) => void; proxies: service.ProxyRow[]; allowAuto?: boolean; currentProfile?: string }

export const freeProxies = (proxies: service.ProxyRow[]) => proxies.filter((p) => !p.rotating && p.usedBy.length === 0).length

/** Proxy picker. Rotating proxies are listed but disabled: they cannot hold an identity.
 *  Labels never carry the proxy username: they are host:port or the user's own name. */
export function ProxySelect({ value, onChange, proxies, allowAuto, currentProfile }: Props) {
  const free = freeProxies(proxies)
  return (
    <select className="select" value={value} onChange={(e) => onChange(e.target.value)} aria-label={t.create.proxy}>
      {value === CHOOSE_PROXY && <option value={CHOOSE_PROXY} disabled>{t.create.proxyChoose}</option>}
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

/** Said wherever a choice leads to a profile without proxy. */
export function RealIpWarning() {
  return <div className="realip" role="note"><Icon name="alert" className="icon sm" /><span>{t.create.realIpWarning}</span></div>
}
