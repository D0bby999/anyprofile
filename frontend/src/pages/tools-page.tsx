// SPDX-License-Identifier: AGPL-3.0-or-later
import { t } from '../i18n'
import { PageHead } from '../components/page-head'
import { PartnerGrid } from '../components/partner-grid'

export function ToolsPage() {
  return (
    <>
      <PageHead title={t.tools.title} sub={t.tools.sub} />
      <div className="pagebody">
        <PartnerGrid kind="tool" />
      </div>
    </>
  )
}
