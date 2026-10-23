// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'react'
import { Api, onEngineProgress, openExternal, service, type EngineProgress } from '../api'
import { t } from '../i18n'
import { errorText, relativeTime } from '../lib/format'
import { tagColor } from '../lib/tags'
import type { useMobile } from '../hooks/use-mobile'
import { EmptyState } from '../components/empty-state'
import { Icon } from '../components/icons'
import { InstallProgress } from '../components/install-progress'
import { MobileDialog } from '../components/mobile-dialog'
import { PageHead } from '../components/page-head'
import { useToast } from '../components/toast'
import { useConfirm } from '../components/confirm'

type Props = { mobile: ReturnType<typeof useMobile>; onOpenPro: () => void; onOpenSettings: () => void }

const busyStates = ['provisioning', 'starting', 'stopping', 'working']
const installReasons = ['android_runtime_missing', 'android_image_missing']
const installEngines = ['android-vm', 'android-vm-image']

function StateChip({ r }: { r: service.MobileRow }) {
  const label = t.mobile.state[r.state] ?? r.state
  if (r.state === 'running') return <span className="chip ok"><span className="dot" />{label}</span>
  if (busyStates.includes(r.state)) return <span className="chip warn"><span className="dot" />{label}</span>
  return <span className="chip">{label}</span>
}

/** Android devices: an AnyProfile Pro feature. Without Pro the page explains
 *  it, and a lapsed subscription still lists, backs up and deletes devices. */
export function MobilePage({ mobile, onOpenPro, onOpenSettings }: Props) {
  const { rows, avail, error, setError, reload } = mobile
  const [proxies, setProxies] = useState<service.ProxyRow[]>([])
  const [editing, setEditing] = useState<service.MobileRow | null | 'new'>(null)
  const [busy, setBusy] = useState('') // a device id while one of its actions runs
  const [installing, setInstalling] = useState<EngineProgress | null>(null)
  useEffect(() => onEngineProgress((p) => { if (installEngines.includes(p.engine)) setInstalling(p) }), [])

  const install = async () => {
    setInstalling({ engine: 'android-vm', done: 0, total: 0 })
    try {
      await Api.InstallMobileRuntime()
      toast.success(t.mobile.installed)
    } catch (e) {
      toast.error(errorText(e))
    } finally {
      setInstalling(null)
      reload()
    }
  }
  const toast = useToast()
  const confirm = useConfirm()

  useEffect(() => { Api.ListProxies().then((p) => setProxies(p ?? [])).catch(() => {}) }, [rows])

  const canUse = !!avail?.pro && !!avail?.ready
  const proxyLabel = (id: string) => {
    if (!id) return <span className="faint">{t.common.direct}</span>
    const p = proxies.find((x) => x.id === id)
    return p ? <span>{p.label}{p.country && <span className="faint"> · {p.country}</span>}</span> : <span className="faint">…</span>
  }

  const run = async (id: string, fn: () => Promise<unknown>, done?: string) => {
    setBusy(id)
    try {
      await fn()
      if (done) toast.success(done)
    } catch (e) {
      toast.error(errorText(e))
    } finally {
      setBusy('')
      reload()
    }
  }

  const open = (r: service.MobileRow) => run(r.id, async () => {
    const res = await Api.StartMobileDevice(r.id)
    if (res.warning === 'ram_tight') toast.info(t.mobile.ramTight)
  })

  const installApps = (id: string) => run(id, async () => {
    const n = await Api.InstallMobileApps(id)
    if (n > 0) toast.success(t.mobile.appsInstalled(n))
  })

  return (
    <>
      <PageHead title={t.mobile.title} sub={t.mobile.total(rows.length)}>
        <button className="btn primary nodrag" disabled={!canUse} onClick={() => setEditing('new')}>
          <Icon name="plus" />{t.mobile.newDevice}
        </button>
      </PageHead>
      <div className="pagebody">
        {avail && !avail.pro && (
          <section className="card pad">
            <h2>{t.mobile.proTitle}</h2>
            <p className="desc">{rows.length ? t.mobile.proLapsed : t.mobile.proText}</p>
            <p className="desc">{t.mobile.proHonest}</p>
            <div className="row"><button className="btn primary" onClick={onOpenPro}><Icon name="spark" />{t.mobile.getPro}</button></div>
          </section>
        )}
        {avail?.pro && !avail.ready && installReasons.includes(avail.reason) && (
          <section className="card pad">
            <h2>{t.mobile.installTitle}</h2>
            <p className="desc">{t.mobile.installText}</p>
            {installing ? <InstallProgress progress={installing} /> : (
              <div className="row"><button className="btn primary" onClick={install}><Icon name="download" />{t.mobile.install}</button></div>
            )}
          </section>
        )}
        {avail?.pro && !avail.ready && !installReasons.includes(avail.reason) && (
          <div className="banner warn" role="status">
            <Icon name="alert" />
            <span className="grow">{t.mobile.notReady} {t.errors[avail.reason] ?? avail.message}</span>
            {avail.supported && <button className="btn sm" onClick={onOpenSettings}>{t.mobile.fix}</button>}
          </div>
        )}
        {error && <div className="banner danger" role="alert"><Icon name="alert" /><span className="grow">{errorText(error)}</span>
          <button className="iconbtn sm" onClick={() => setError('')} aria-label={t.common.close}><Icon name="x" /></button></div>}

        <section className="card fill" style={{ minHeight: 220 }}>
          <div className="tablewrap">
            {rows.length === 0 ? (
              <EmptyState icon="mobile" title={t.mobile.emptyTitle} text={canUse ? t.mobile.emptyText : undefined}>
                {canUse && <button className="btn primary" onClick={() => setEditing('new')}><Icon name="plus" />{t.mobile.newDevice}</button>}
              </EmptyState>
            ) : (
              <table className="table">
                <thead>
                  <tr><th>{t.mobile.colName}</th><th>{t.mobile.colProxy}</th><th>{t.mobile.colState}</th>
                    <th>{t.mobile.colLastOpen}</th><th className="col-actions" /></tr>
                </thead>
                <tbody>
                  {rows.map((r) => {
                    const working = busy === r.id || busyStates.includes(r.state)
                    const running = r.state === 'running'
                    return (
                      <tr key={r.id}>
                        <td className="cellname">
                          <b className="selectable">{r.name}</b>
                          <span className="sub">#{r.serial} · {r.ramMb / 1024} GB · {r.cpus} CPU</span>
                          {r.tags.length > 0 && <span className="row">{r.tags.map((tag) => (
                            <span key={tag} className="chip"><span className="tagdot" style={{ background: `var(--tag-${tagColor(tag)})` }} />{tag}</span>
                          ))}</span>}
                          {r.error && <small className="danger">{r.error}</small>}
                        </td>
                        <td>{proxyLabel(r.proxyId)}</td>
                        <td className="stack nowrap"><StateChip r={r} />
                          {r.backedUpAt && <small className="faint">{t.mobile.backedUp(relativeTime(r.backedUpAt))}</small>}</td>
                        <td className="muted nowrap">{r.lastOpenAt ? relativeTime(r.lastOpenAt) : t.mobile.never}</td>
                        <td className="col-actions"><span className="row">
                          {running && (
                            <button className="iconbtn sm" onClick={() => run(r.id, () => Api.ShowMobileDevice(r.id))} aria-label={t.mobile.show} title={t.mobile.show}><Icon name="monitor" /></button>
                          )}
                          {running && (
                            <button className="btn sm" disabled={working} onClick={() => installApps(r.id)}><Icon name="plus" className="icon sm" />{t.mobile.installApps}</button>
                          )}
                          <button className="iconbtn sm" disabled={working} onClick={() => setEditing(r)} aria-label={t.mobile.edit} title={t.mobile.edit}><Icon name="pencil" /></button>
                          <button className="iconbtn sm" disabled={working || running} onClick={() => run(r.id, () => Api.BackupMobileDevice(r.id), t.mobile.backupDone)}
                            aria-label={t.mobile.backup} title={t.mobile.backup}><Icon name="download" /></button>
                          {r.backedUpAt && (
                            <button className="iconbtn sm" disabled={working || running} aria-label={t.mobile.restoreBackup} title={t.mobile.restoreBackup}
                              onClick={async () => { if (await confirm({ ...t.confirm.restoreMobile(relativeTime(r.backedUpAt)), danger: true })) run(r.id, () => Api.RestoreMobileBackup(r.id), t.mobile.restoreDone) }}>
                              <Icon name="restore" /></button>
                          )}
                          <button className="iconbtn sm" disabled={working} onClick={() => run(r.id, () => Api.TrashMobileDevice(r.id), t.mobile.trashed)}
                            aria-label={t.mobile.trash} title={t.mobile.trash}><Icon name="trash" /></button>
                          {running || r.state === 'stopping' ? (
                            <button className={`btn sm stop actbtn${working ? ' busy' : ''}`} aria-busy={working} disabled={working} onClick={() => run(r.id, () => Api.StopMobileDevice(r.id))}>
                              <Icon name={working ? 'refresh' : 'stop'} className="icon sm" />{t.mobile.close}
                            </button>
                          ) : (
                            <button className={`btn sm open actbtn${working ? ' busy' : ''}`} aria-busy={working} disabled={working || !canUse} onClick={() => open(r)}>
                              <Icon name={working ? 'refresh' : 'play'} className="icon sm" />{t.mobile.open}
                            </button>
                          )}
                        </span></td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            )}
          </div>
        </section>
        {rows.length > 0 && canUse && (
          <p className="hint">{t.mobile.apkHint} {t.mobile.apkWhere}{' '}
            <a href="#" onClick={(e) => { e.preventDefault(); openExternal('https://www.apkmirror.com/') }}>{t.mobile.apkMirror}</a>{' '}
            {t.mobile.apkOr}{' '}
            <a href="#" onClick={(e) => { e.preventDefault(); openExternal('https://f-droid.org/') }}>{t.mobile.fdroid}</a>.
          </p>
        )}
      </div>
      {editing && (
        <MobileDialog row={editing === 'new' ? null : editing} onClose={() => setEditing(null)}
          onSaved={(name) => { const created = editing === 'new'; setEditing(null); reload(); if (created) toast.success(t.mobile.created(name)) }} />
      )}
    </>
  )
}
