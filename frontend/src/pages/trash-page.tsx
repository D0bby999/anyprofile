// SPDX-License-Identifier: AGPL-3.0-or-later
import { useCallback, useEffect, useState } from 'react'
import { Api, service, store } from '../api'
import { t } from '../i18n'
import { errorText, relativeTime } from '../lib/format'
import { machineLabel } from '../lib/persona'
import { useSites } from '../hooks/use-sites'
import { EmptyState } from '../components/empty-state'
import { Icon } from '../components/icons'
import { SiteBadges } from '../components/site-glyph'
import { PageHead } from '../components/page-head'
import { useToast } from '../components/toast'
import { useConfirm } from '../components/confirm'

export function TrashPage({ onChanged }: { onChanged: () => void }) {
  const [rows, setRows] = useState<service.ProfileRow[]>([])
  const [devices, setDevices] = useState<service.MobileRow[]>([])
  const [error, setError] = useState('')
  const { sites } = useSites()
  const toast = useToast()
  const confirm = useConfirm()

  const load = useCallback(() => {
    const trashed = store.ProfileFilter.createFrom({ query: '', tag: '', trashed: true })
    Api.ListProfiles(trashed).then((r) => setRows(r ?? [])).catch((e) => setError(errorText(e)))
    Api.ListMobileDevices(trashed).then((r) => setDevices(r ?? [])).catch((e) => setError(errorText(e)))
  }, [])
  useEffect(load, [load])

  const run = async (fn: () => Promise<unknown>) => {
    try {
      await fn()
    } catch (e) {
      toast.error(errorText(e))
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
            {rows.length === 0 && devices.length === 0 ? <EmptyState icon="trash" title={t.trash.emptyTitle} text={t.trash.emptyText} /> : rows.length > 0 && (
              <table className="table">
                {devices.length > 0 && <caption className="tablecap">{t.trash.profilesTitle}</caption>}
                <tbody>
                  {rows.map((r) => (
                    <tr key={r.id}>
                      <td className="cellname"><b>{r.name}</b><span className="sub">{machineLabel(r)}</span></td>
                      <td><SiteBadges ids={r.sites} sites={sites} /></td>
                      <td className="muted">{t.trash.deletedAgo(relativeTime(r.deletedAt))}</td>
                      <td className="col-actions"><span className="row">
                        <button className="btn sm" onClick={() => run(() => Api.RestoreProfile(r.id))}><Icon name="restore" className="icon sm" />{t.trash.restore}</button>
                        <button className="btn sm danger" onClick={async () => { if (await confirm({ ...t.confirm.purge(r.name), danger: true })) run(() => Api.PurgeProfile(r.id)) }}>
                          <Icon name="trash" className="icon sm" />{t.trash.purge}
                        </button>
                      </span></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
            {devices.length > 0 && (
              <table className="table">
                <caption className="tablecap">{t.trash.mobileTitle}</caption>
                <tbody>
                  {devices.map((d) => (
                    <tr key={d.id}>
                      <td className="cellname"><b>{d.name}</b><span className="sub">#{d.serial} · {d.ramMb / 1024} GB</span></td>
                      <td className="muted">{t.trash.deletedAgo(relativeTime(d.deletedAt))}</td>
                      <td className="col-actions"><span className="row">
                        <button className="btn sm" onClick={() => run(() => Api.RestoreMobileDevice(d.id))}><Icon name="restore" className="icon sm" />{t.trash.restore}</button>
                        <button className="btn sm danger" onClick={async () => { if (await confirm({ ...t.confirm.purgeMobile(d.name), danger: true })) run(() => Api.PurgeMobileDevice(d.id)) }}>
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
