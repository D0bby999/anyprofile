// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useRef, useState } from 'react'
import { Api, onEngineProgress, openExternal, service, type EngineProgress } from '../api'
import { t } from '../i18n'
import { errorText, relativeTime } from '../lib/format'
import { tagColor } from '../lib/tags'
import type { useMobile } from '../hooks/use-mobile'
import { EmptyState } from '../components/empty-state'
import { Icon } from '../components/icons'
import { InstallProgress } from '../components/install-progress'
import { MOBILE_APPS_GUIDE } from '../components/api-settings'
import { MobileDialog } from '../components/mobile-dialog'
import { MobileViewer } from '../components/mobile-viewer'
import { PageHead } from '../components/page-head'
import { useToast } from '../components/toast'
import { useConfirm } from '../components/confirm'

type Props = { mobile: ReturnType<typeof useMobile>; onOpenPro: () => void; onOpenSettings: () => void }

const busyStates = ['provisioning', 'starting', 'stopping', 'working']
const installReasons = ['anymobile_runtime_missing', 'anymobile_image_missing']
const installEngines = ['anymobile', 'anymobile-image']

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
  const [fastStart, setFastStart] = useState(true)
  const [recording, setRecording] = useState<Set<string>>(new Set())
  const [viewing, setViewing] = useState<service.MobileRow | null>(null)
  useEffect(() => onEngineProgress((p) => { if (installEngines.includes(p.engine)) setInstalling(p) }), [])

  const install = async () => {
    setInstalling({ engine: 'anymobile', done: 0, total: 0 })
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
  useEffect(() => { Api.MobileFastStart().then(setFastStart).catch(() => {}) }, [])

  const canUse = !!avail?.pro && !!avail?.ready
  // Nothing installed and nothing in the signed list: the runtime is not out yet.
  const unpublished = !!avail?.install && !avail.install.runtime.installed && !avail.install.runtime.latest
  const proxyLabel = (r: service.MobileRow) => {
    if (!r.proxyId) return <span className="proxy-pill"><Icon name="direct" className="icon xs" /><span className="faint">{t.common.direct}</span></span>
    const p = proxies.find((x) => x.id === r.proxyId)
    if (!p) return <span className="proxy-pill"><Icon name="proxy" className="icon xs" /><span className="faint">…</span></span>
    return (
      <span className="proxy-pill">
        <Icon name="proxy" className="icon xs" />
        <span>{p.label}{p.country && <span className="faint"> · {p.country}</span>}</span>
        {p.failedAt && <small className="danger">{r.state === 'running' ? t.mobile.proxyDown : t.proxies.failed}</small>}
      </span>
    )
  }

  // The trash gives a device back, so moving it there asks nothing and offers Undo.
  const trash = (r: service.MobileRow) => run(r.id, async () => {
    await Api.TrashMobileDevice(r.id)
    toast.success(t.profiles.trashedOne(r.name), { label: t.common.undo,
      run: () => { Api.RestoreMobileDevice(r.id).then(reload).catch((e) => toast.error(errorText(e))) } })
  })

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

  const screenshot = (id: string) => run(id, async () => {
    const path = await Api.ScreenshotMobileDevice(id)
    toast.success(t.mobile.screenshotSaved(path))
  })

  const toggleRecording = (id: string) => {
    if (recording.has(id)) {
      run(id, async () => {
        const path = await Api.StopMobileRecording(id)
        setRecording((prev) => { const next = new Set(prev); next.delete(id); return next })
        toast.success(t.mobile.recordingSaved(path))
      })
      return
    }
    run(id, async () => {
      await Api.StartMobileRecording(id)
      setRecording((prev) => new Set(prev).add(id))
      toast.success(t.mobile.recordingStarted)
    })
  }

  const toggleTop = (id: string) => run(id, async () => {
    const on = await Api.MobileWindowOnTop(id)
    await Api.SetMobileWindowOnTop(id, !on)
    toast.success(on ? t.mobile.topOff : t.mobile.topOn)
  })

  const installApps = (id: string) => run(id, async () => {
    const n = await Api.InstallMobileApps(id)
    if (n > 0) toast.success(t.mobile.appsInstalled(n))
  })

  const installAurora = (id: string) => run(id, () => Api.InstallMobileAurora(id), t.mobile.auroraInstalled)

  return (
    <>
      <PageHead title={t.mobile.title} badge={t.mobile.beta} sub={t.mobile.total(rows.length)}>
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
            <p className="desc">{unpublished ? t.mobile.comingSoon : t.mobile.installText}</p>
            {unpublished ? null : installing ? <InstallProgress progress={installing} /> : (
              <div className="row"><button className="btn primary" onClick={install}><Icon name="download" />{t.mobile.install}</button></div>
            )}
          </section>
        )}
        {avail?.pro && !avail.ready && avail.reason === 'ldplayer_missing' && (
          <section className="card pad">
            <h2>{t.mobile.ldplayerTitle}</h2>
            <p className="desc">{t.mobile.ldplayerText}</p>
            <p className="desc">{t.mobile.ldplayerLite}</p>
            <div className="row">
              <button className="btn primary" onClick={() => openExternal('https://www.ldplayer.net/')}><Icon name="external" />{t.mobile.ldplayerDownload}</button>
              <button className="btn" onClick={reload}><Icon name="refresh" />{t.mobile.ldplayerCheck}</button>
            </div>
          </section>
        )}
        {avail?.pro && !avail.ready && avail.reason !== 'ldplayer_missing' && !installReasons.includes(avail.reason) && (
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
              <table className="table mobile-table">
                <colgroup>
                  <col className="mob-col-name" />
                  <col className="mob-col-proxy" />
                  <col className="mob-col-state" />
                  <col className="mob-col-last" />
                  <col className="mob-col-actions" />
                </colgroup>
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
                          <span className="mobile-device-cell">
                          <span className="mobile-device-avatar"><Icon name="mobile" /></span>
                          <span className="mobile-device-copy">
                            <b className="selectable mobile-name" title={r.name}>{r.name}</b>
                            <span className="sub mobile-meta">#{r.serial} · {r.ramMb / 1024} GB · {r.cpus} CPU</span>
                            {r.tags.length > 0 && <span className="row mobile-tags">{r.tags.map((tag) => (
                              <span key={tag} className="chip"><span className="tagdot" style={{ background: `var(--tag-${tagColor(tag)})` }} />{tag}</span>
                            ))}</span>}
                            {r.error && <small className="danger">{r.error}</small>}
                          </span>
                          </span>
                        </td>
                        <td className="stack mobile-proxy">{proxyLabel(r)}</td>
                        <td className="stack mobile-state"><StateChip r={r} />
                          {r.state === 'starting' && mobile.phases?.[r.id] && <small role="status" className="faint">{t.mobile.startPhase[mobile.phases[r.id]]}</small>}
                          {r.backedUpAt && <small className="faint">{t.mobile.backedUp(relativeTime(r.backedUpAt))}</small>}</td>
                        <td className="muted nowrap mobile-last">{r.lastOpenAt ? relativeTime(r.lastOpenAt) : t.mobile.never}</td>
                        <td className="col-actions"><span className="row mobile-actions">
                          {running && (
                            <button className="iconbtn sm" disabled={working} onClick={() => installApps(r.id)} aria-label={t.mobile.installApps} title={t.mobile.installApps}>
                              <Icon name="download" />
                            </button>
                          )}
                          <button className={`iconbtn sm${running ? '' : ' slot'}`} disabled={!running}
                            onClick={() => run(r.id, () => Api.ShowMobileDevice(r.id))} aria-label={t.mobile.show} title={running ? t.mobile.show : undefined}
                            aria-hidden={!running || undefined} tabIndex={running ? undefined : -1}><Icon name="monitor" /></button>
                          <MobileRowMenu row={r} running={running} busy={working} recording={recording.has(r.id)}
                            onAurora={() => installAurora(r.id)}
                            onView={() => setViewing(r)}
                            onScreenshot={() => screenshot(r.id)} onRecord={() => toggleRecording(r.id)}
                            onTop={() => toggleTop(r.id)} onEdit={() => setEditing(r)}
                            onBackup={() => run(r.id, () => Api.BackupMobileDevice(r.id), t.mobile.backupDone)}
                            onRestore={async () => { if (await confirm({ ...t.confirm.restoreMobile(relativeTime(r.backedUpAt)), danger: true })) run(r.id, () => Api.RestoreMobileBackup(r.id), t.mobile.restoreDone) }}
                            onTrash={() => trash(r)} />
                          {running || r.state === 'stopping' ? (
                            <button className={`btn sm stop actbtn${working ? ' busy' : ''}`} aria-busy={working} disabled={working} onClick={() => run(r.id, () => Api.StopMobileDevice(r.id))}>
                              <Icon name={working ? 'refresh' : 'stop'} className="icon sm" /><MobileActLabel shown={t.mobile.close} />
                            </button>
                          ) : (
                            <button className={`btn sm open actbtn${working ? ' busy' : ''}`} aria-busy={working} disabled={working || !canUse} onClick={() => open(r)}>
                              <Icon name={working ? 'refresh' : 'play'} className="icon sm" /><MobileActLabel shown={t.mobile.open} />
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
        {rows.length > 0 && canUse && avail?.driver === 'vm' && (
          <div className="mobile-faststart">
            <label className="row checkrow">
              <input type="checkbox" checked={fastStart} onChange={(e) => {
                const on = e.target.checked
                setFastStart(on)
                Api.SetMobileFastStart(on).catch((err) => { setFastStart(!on); toast.error(errorText(err)) })
              }} />
              {t.mobile.fastStart}
            </label>
            <span className="hint">{t.mobile.fastStartHint}</span>
          </div>
        )}
        {rows.length > 0 && canUse && (
          <section className="mobile-help" aria-label={t.mobile.apkHelp}>
            <strong>{t.mobile.apkHelp}</strong>
            <p>{t.mobile.apkHint} {avail?.driver === 'vm' && t.mobile.apkWhere}</p>
            <p><a href="https://f-droid.org/packages/com.aurora.store/" onClick={(e) => { e.preventDefault(); openExternal('https://f-droid.org/packages/com.aurora.store/') }}>Aurora Store</a>{' · '}
              <a href="https://f-droid.org/F-Droid.apk" onClick={(e) => { e.preventDefault(); openExternal('https://f-droid.org/F-Droid.apk') }}>F-Droid</a>{' · '}
              <a href={MOBILE_APPS_GUIDE} onClick={(e) => { e.preventDefault(); openExternal(MOBILE_APPS_GUIDE) }}>{t.mobile.apkGuide}</a>
            </p>
          </section>
        )}
      </div>
      {viewing && <MobileViewer row={viewing} onClose={() => setViewing(null)} />}
      {editing && (
        <MobileDialog row={editing === 'new' ? null : editing} onClose={() => setEditing(null)}
          onSaved={(name) => { const created = editing === 'new'; setEditing(null); reload(); if (created) toast.success(t.mobile.created(name)) }} />
      )}
    </>
  )
}


/** The row's controls stay one action + one overflow menu, like the profile table. */
function MobileRowMenu({ row, running, busy, recording, onAurora, onView, onScreenshot, onRecord, onTop, onEdit, onBackup, onRestore, onTrash }: {
  row: service.MobileRow; running: boolean; busy: boolean; recording: boolean
  onAurora: () => void; onView: () => void; onScreenshot: () => void; onRecord: () => void; onTop: () => void
  onEdit: () => void; onBackup: () => void; onRestore: () => void; onTrash: () => void
}) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => { if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false) }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [open])
  const act = (fn: () => void) => { setOpen(false); fn() }
  return (
    <div className="rowmenu-wrap" ref={ref}>
      <button type="button" className="iconbtn sm" aria-haspopup="menu" aria-expanded={open} aria-label={t.menu.more}
        disabled={busy} onClick={() => setOpen(!open)}><Icon name="more" /></button>
      {open && (
        <div role="menu" className="rowmenu">
          {running && <button role="menuitem" onClick={() => act(onAurora)}>{t.mobile.installAurora}</button>}
          {running && <button role="menuitem" onClick={() => act(onView)}>{t.mobile.viewInApp}</button>}
          {running && <button role="menuitem" onClick={() => act(onScreenshot)}>{t.mobile.screenshot}</button>}
          {running && <button role="menuitem" onClick={() => act(onRecord)}>{recording ? t.mobile.stopRecording : t.mobile.recordScreen}</button>}
          {running && <button role="menuitem" onClick={() => act(onTop)}>{t.mobile.alwaysOnTop}</button>}
          <button role="menuitem" onClick={() => act(onEdit)}>{t.mobile.edit}</button>
          <button role="menuitem" disabled={running} onClick={() => act(onBackup)}>{t.mobile.backup}</button>
          {row.backedUpAt && <button role="menuitem" disabled={running} onClick={() => act(onRestore)}>{t.mobile.restoreBackup}</button>}
          <hr />
          <button role="menuitem" onClick={() => act(onTrash)}>{t.mobile.trash}</button>
        </div>
      )}
    </div>
  )
}

/** Fixed-width action label: Open and Close do not resize the row. */
function MobileActLabel({ shown }: { shown: string }) {
  const all = [...new Set([t.mobile.open, t.mobile.close, shown])]
  return <span className="actlabel">{all.map((x) => <span key={x} aria-hidden={x === shown ? undefined : true} className={x === shown ? undefined : 'off'}>{x}</span>)}</span>
}
