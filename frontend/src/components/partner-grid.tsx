// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { openExternal } from '../api'
import { language, t } from '../i18n'
import { partnerLink, partnerLogo, partnersOf, proxyTypes, type Partner, type ProxyType } from '../lib/partners'
import { Dialog } from './dialog'
import { Icon } from './icons'

/** One tile per partner; a click opens its site in the default browser. */
export function PartnerGrid({ items }: { items: Partner[] }) {
  return (
    <ul className="partnergrid">
      {items.map((p) => {
        const logo = partnerLogo(p)
        return (
          <li key={p.id}>
            <button type="button" className="partner" onClick={() => openExternal(partnerLink(p, language))}>
              {logo
                ? <img className="partner-mark" src={logo} alt="" draggable={false} />
                : <span className="partner-mark letter" aria-hidden="true">{p.name[0]}</span>}
              <span className="partner-main">
                <b>{p.name}</b>
                <span className="muted">{t.partners.desc[p.id]}</span>
                {(p.types || p.own || p.openSource) && (
                  <span className="partner-tags">
                    {p.own && <span className="chip accent">{t.partners.ours}</span>}
                    {p.openSource && <span className="chip">{t.partners.openSource}</span>}
                    {p.types?.map((k) => <span key={k} className="chip">{t.partners.types[k]}</span>)}
                  </span>
                )}
              </span>
              <Icon name="external" className="icon sm" />
            </button>
          </li>
        )
      })}
    </ul>
  )
}

/** "Buy proxies" with its own dialog, for the places a user finds out they need a proxy. */
export function BuyProxiesButton({ className = 'btn buy' }: { className?: string }) {
  const [open, setOpen] = useState(false)
  return (
    <>
      <button type="button" className={className} onClick={() => setOpen(true)}><Icon name="cart" />{t.proxies.buy}</button>
      {open && <BuyProxyDialog onClose={() => setOpen(false)} />}
    </>
  )
}

export function BuyProxyDialog({ onClose }: { onClose: () => void }) {
  const [type, setType] = useState<ProxyType | ''>('')
  const shown = partnersOf('proxy').filter((p) => !type || p.types?.includes(type))
  return (
    <Dialog title={t.proxies.buy} icon="cart" onClose={onClose} className="partnerdialog"
      footer={<button type="button" className="btn" onClick={onClose}>{t.common.close}</button>}>
      <p className="partner-guide muted">{t.partners.guide}</p>
      <div className="seg" role="group" aria-label={t.partners.filter}>
        {(['', ...proxyTypes] as const).map((k) => (
          <button key={k || 'all'} type="button" aria-pressed={type === k} onClick={() => setType(k)}>
            {k ? t.partners.types[k] : t.partners.all}
          </button>
        ))}
      </div>
      <PartnerGrid items={shown} />
      <p className="partner-note faint">{t.partners.disclosure}</p>
    </Dialog>
  )
}
