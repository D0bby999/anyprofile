// SPDX-License-Identifier: AGPL-3.0-or-later
import { service } from '../api'
import { t } from '../i18n'

type Props = { value: string; onChange: (v: string) => void; proxies: service.ProxyRow[]; allowAuto?: boolean; currentProfile?: string }

/** Proxy picker. Rotating proxies are listed but disabled: they cannot hold an identity. */
export function ProxySelect({ value, onChange, proxies, allowAuto, currentProfile }: Props) {
  const free = proxies.some((p) => !p.rotating && p.usedBy.length === 0)
  return (
    <select className="select" value={value} onChange={(e) => onChange(e.target.value)}>
      {allowAuto && <option value="auto">{free ? t.create.proxyAuto : t.create.proxyAutoNone}</option>}
      <option value="">{t.common.directLong}</option>
      {proxies.map((p) => {
        const others = p.usedBy.filter((n) => n !== currentProfile)
        const where = [p.country, p.exitIp].filter(Boolean).join(' ')
        const note = p.rotating ? t.proxies.rotating : others.length ? others.join(', ') : ''
        return (
          <option key={p.id} value={p.id} disabled={p.rotating}>
            {p.label}{where ? `  ·  ${where}` : ''}{note ? `  (${note})` : ''}
          </option>
        )
      })}
    </select>
  )
}
