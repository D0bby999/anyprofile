// SPDX-License-Identifier: AGPL-3.0-or-later
import { useCallback, useEffect, useState } from 'react'
import { Api, service, store } from '../api'
import { t } from '../i18n'
import { errorText, relativeTime } from '../lib/format'
import { personaLabel } from '../lib/persona'
import { siteById } from '../lib/sites'
import { useSites } from '../hooks/use-sites'
import { EmptyState } from '../components/empty-state'
import { Icon } from '../components/icons'
import { SiteGlyph } from '../components/site-glyph'
import { PageHead } from '../components/page-head'

export function TrashPage({ onChanged }: { onChanged: () => void }) {
  const [rows, setRows] = useState<service.ProfileRow[]>([])
  const [error, setError] = useState('')
  const { sites } = useSites()

  const load = useCallback(() => {
    Api.ListProfiles(store.ProfileFilter.createFrom({ query: '', tag: '', trashed: true }))
      .then((r) => setRows(r ?? [])).catch((e) => setError(errorText(e)))
  }, [])
  useEffect(load, [load])

  const run = async (fn: () => Promise<unknown>) => {
    setError('')
    try {
      await fn()
    } catch (e) {
      setError(errorText(e))
    }
    load()
    onChanged()
  }

  return (
    <>
      <PageHead title={t.trash.title} sub={t.trash.total(rows.length)} />
      <div className="pagebody">
        {error && <div className="banner danger" role="alert"><Icon name="alert" /><span className="grow">{error}</span></div>}
        <section className="card fill">
          <div className="tablewrap">
            {rows.length === 0 ? <EmptyState icon="trash" title={t.trash.emptyTitle} text={t.trash.emptyText} /> : (
              <table className="table">
                <tbody>
                  {rows.map((r) => (
                    <tr key={r.id}>
                      <td className="cellname"><b>{r.name}</b><span className="sub">{personaLabel(r.personaId)}</span></td>
                      <td><span className="platform"><SiteGlyph site={siteById(sites, r.site)} />{siteById(sites, r.site).name}</span></td>
                      <td className="muted">{t.trash.deletedAgo(relativeTime(r.deletedAt))}</td>
                      <td className="col-actions"><span className="row">
                        <button className="btn sm" onClick={() => run(() => Api.RestoreProfile(r.id))}><Icon name="restore" className="icon sm" />{t.trash.restore}</button>
                        <button className="btn sm danger" onClick={() => window.confirm(t.trash.confirmPurge(r.name)) && run(() => Api.PurgeProfile(r.id))}>
                          <Icon name="trash" className="icon sm" />{t.trash.purge}
                        </button>
                      </span></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </section>
      </div>
    </>
  )
}
