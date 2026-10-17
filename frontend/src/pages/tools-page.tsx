// SPDX-License-Identifier: AGPL-3.0-or-later
import { t } from '../i18n'
import { PageHead } from '../components/page-head'
import { PartnerGrid } from '../components/partner-grid'
import { partnersOf, toolGroups } from '../lib/partners'

export function ToolsPage() {
  const tools = partnersOf('tool')
  return (
    <>
      <PageHead title={t.tools.title} sub={t.tools.sub} />
      <div className="pagebody">
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
