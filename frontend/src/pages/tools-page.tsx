// SPDX-License-Identifier: AGPL-3.0-or-later
import { t } from '../i18n'
import { Icon } from '../components/icons'
import { PageHead } from '../components/page-head'
import { PartnerGrid } from '../components/partner-grid'
import { partnersOf, toolGroups } from '../lib/partners'

/** onOpenPro goes to Settings → Pro, where the Pro pack is bought and managed. */
export function ToolsPage({ onOpenPro }: { onOpenPro?: () => void }) {
  const tools = partnersOf('tool')
  return (
    <>
      <PageHead title={t.tools.title} sub={t.tools.sub} />
      <div className="pagebody">
        {onOpenPro && (
          <div className="banner pro-banner">
            <Icon name="spark" />
            <span className="grow"><b>{t.tools.proTitle}</b> {t.tools.proBody}</span>
            <button type="button" className="btn sm primary" onClick={onOpenPro}>{t.tools.proOpen}</button>
          </div>
        )}
        {toolGroups.map((g) => {
          const items = tools.filter((p) => p.group === g)
          return items.length > 0 && (
            <section key={g} className="partnersection">
              <h2>{t.partners.groups[g]}</h2>
              <PartnerGrid items={items} />
            </section>
          )
        })}
      </div>
    </>
  )
}
