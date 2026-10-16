// SPDX-License-Identifier: AGPL-3.0-or-later
import { openExternal } from '../api'
import { language, t } from '../i18n'
import { partnerLink, partnersOf, type PartnerKind } from '../lib/partners'
import { Dialog } from './dialog'
import { Icon } from './icons'

/** One tile per partner; a click opens its site in the default browser. */
export function PartnerGrid({ kind }: { kind: PartnerKind }) {
  return (
    <ul className="partnergrid">
      {partnersOf(kind).map((p) => (
        <li key={p.id}>
          <button type="button" className="partner" onClick={() => openExternal(partnerLink(p, language))}>
            <span className="partner-mark" aria-hidden="true">{p.name[0]}</span>
            <span className="partner-main">
              <b>{p.name}</b>
              <span className="muted">{t.partners.desc[p.id]}</span>
            </span>
            <Icon name="external" className="icon sm" />
          </button>
        </li>
      ))}
    </ul>
  )
}

export function BuyProxyDialog({ onClose }: { onClose: () => void }) {
  return (
    <Dialog title={t.proxies.buy} icon="cart" onClose={onClose} className="partnerdialog"
      footer={<button type="button" className="btn" onClick={onClose}>{t.common.close}</button>}>
      <PartnerGrid kind="proxy" />
      <p className="partner-note faint">{t.partners.disclosure}</p>
    </Dialog>
  )
}
