// SPDX-License-Identifier: AGPL-3.0-or-later
import { openExternal } from '../api'
import { t } from '../i18n'
import { shownDonateLinks } from '../lib/donate'
import { Icon } from './icons'

/** Settings → Support AnyProfile: one button per donation link, opened in the default browser. */
export function DonateSettings() {
  const links = shownDonateLinks()
  return (
    <div className="field">
      <span className="row">
        {links.map((l) => (
          <button key={l.id} type="button" className="btn sm" onClick={() => openExternal(l.url)}>
            <Icon name="heart" className="icon sm" />{t.donate[l.id]}
          </button>
        ))}
      </span>
      {links.some((l) => l.id === 'stripe') && <span className="hint">{t.donate.stripeNote}</span>}
    </div>
  )
}
