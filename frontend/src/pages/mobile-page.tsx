// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useMemo, useRef, useState } from 'react'
import { Api, onEngineProgress, openExternal, service, type EngineProgress } from '../api'
import { t } from '../i18n'
import { errorText, fold, relativeTime } from '../lib/format'
import { groupOf, mergeOrder, moveTo, rank } from '../lib/profile-order'
import { rangeSelect } from '../lib/selection'
import { tagColor } from '../lib/tags'
import type { useMobile } from '../hooks/use-mobile'
import { EmptyState } from '../components/empty-state'
import { Icon } from '../components/icons'
import { InstallProgress } from '../components/install-progress'
import { MOBILE_APPS_GUIDE } from '../components/api-settings'
import { MobileDialog } from '../components/mobile-dialog'
import { MobileDrawer, StateChip } from '../components/mobile-drawer'
import { MobileViewer } from '../components/mobile-viewer'
import { PageHead } from '../components/page-head'
import { useToast } from '../components/toast'
import { useConfirm } from '../components/confirm'
import { useRowDrag } from '../components/use-row-drag'

type Props = { mobile: ReturnType<typeof useMobile>; onOpenPro: () => void; onOpenSettings: () => void }

const busyStates = ['provisioning', 'starting', 'stopping', 'working']
const installReasons = ['anymobile_runtime_missing', 'anymobile_image_missing']
const installEngines = ['anymobile', 'anymobile-image']
const CONTROLS = 'input, button, a, select, textarea, label, [role="menu"], [role="menuitem"]'
const MOBILE_ORDER_KEY = 'anyprofile.mobileOrder'

function readMobileOrder(): Map<string, number> {
  try {
    const raw = localStorage.getItem(MOBILE_ORDER_KEY)
    if (!raw) return new Map()
    const list: string[] = JSON.parse(raw)
    return new Map(list.map((id, i) => [id, i + 1]))
  } catch {
    return new Map()
  }
}

function writeMobileOrder(order: string[]) {
  try {
    localStorage.setItem(MOBILE_ORDER_KEY, JSON.stringify(order))
  } catch {
    // ignore
  }
}

/** Android devices: an AnyProfile Pro feature. Without Pro the page explains
 *  it, and a lapsed subscription still lists, backs up and deletes devices. */
export function MobilePage({ mobile, onOpenPro, onOpenSettings }: Props) {
  const { rows, avail, error, setError, reload } = mobile
  const [proxies, setProxies] = useState<service.ProxyRow[]>([])
  const [editing, setEditing] = useState<'new' | null>(null)
  const [activeId, setActiveId] = useState<string | null>(null)
  const [query, setQuery] = useState('')
  const [busy, setBusy] = useState('') // a device id while one of its actions runs
  const [installing, setInstalling] = useState<EngineProgress | null>(null)
  const [fastStart, setFastStart] = useState(true)
  const [recording, setRecording] = useState<Set<string>>(new Set())
  const [viewing, setViewing] = useState<service.MobileRow | null>(null)
  const [order, setOrder] = useState<Map<string, number>>(readMobileOrder)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [lastToggled, setLastToggled] = useState<string | null>(null)
  useEffect(() => onEngineProgress((p) => { if (installEngines.includes(p.engine)) setInstalling(p) }), [])

  const active = useMemo(() => rows.find((r) => r.id === activeId) ?? null, [rows, activeId])
  const allTags = useMemo(() => [...new Set(rows.flatMap((r) => r.tags))], [rows])

  const visibleRows = useMemo(() => {
    const q = fold(query.trim())
    const serial = /^#(\d+)$/.exec(q)?.[1]
    const list = rows.filter((r) => {
      if (!q) return true
      if (serial) return String(r.serial) === serial
      const p = proxies.find((x) => x.id === r.proxyId)
      return [r.name, r.notes, r.preset, p?.label, p?.exitIp, p?.country, ...r.tags].some((x) => fold(x ?? '').includes(q))
    })
    list.sort((a, b) => Number(b.pinned) - Number(a.pinned) || (rank(a, order) - rank(b, order)))
    return list
  }, [rows, query, proxies, order])

  const reorder = (shown: string[]) => {
    const whole = [...rows].sort((a, b) => Number(b.pinned) - Number(a.pinned) || (rank(a, order) - rank(b, order))).map((r) => r.id)
    const next = mergeOrder(whole, shown)
    setOrder(new Map(next.map((id, i) => [id, i + 1])))
    writeMobileOrder(next)
  }

  const body = useRef<HTMLTableSectionElement>(null)
  const { dragging, press } = useRowDrag(body, visibleRows, reorder)
  const regrip = useRef<string | null>(null)
  useEffect(() => {
    if (!regrip.current) return
    body.current?.querySelector<HTMLElement>(`[data-grip="${regrip.current}"]`)?.focus()
    regrip.current = null
  })

  const keyMove = (e: React.KeyboardEvent, id: string) => {
    const step = e.key === 'ArrowUp' ? -1 : e.key === 'ArrowDown' ? 1 : 0
    if (!step) return
    e.preventDefault()
    e.stopPropagation()
    const ids = visibleRows.map((r) => r.id)
    const i = ids.indexOf(id)
    const [lo, hi] = groupOf(visibleRows, i)
    if (i + step < lo || i + step > hi) return
    regrip.current = id
    reorder(moveTo(ids, id, i + step))
  }

  const toggle = (id: string, shift = false) => {
    setSelected((cur) => {
      if (shift && lastToggled) {
        return rangeSelect(visibleRows.map((r) => r.id), lastToggled, id, cur)
      }
      const next = new Set(cur)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
    setLastToggled(id)
  }
  const allSelected = visibleRows.length > 0 && visibleRows.every((r) => selected.has(r.id))
  const toggleAll = () => setSelected((cur) => (visibleRows.every((r) => cur.has(r.id)) ? new Set() : new Set(visibleRows.map((r) => r.id))))

  const trashSelected = async () => {
    const ids = [...selected]
    if (ids.length === 0) return
    setBusy('bulk')
    try {
      for (const id of ids) {
        await Api.TrashMobileDevice(id)
      }
      setSelected(new Set())
      toast.success(t.profiles.trashed(ids.length), {
        label: t.common.undo,
        run: async () => {
          for (const id of ids) {
            await Api.RestoreMobileDevice(id).catch(() => {})
          }
          reload()
        },
      })
    } catch (e) {
      toast.error(errorText(e))
    } finally {
      setBusy('')
      reload()
    }
  }

  useEffect(() => {
    setSelected((cur) => {
      if (cur.size === 0) return cur
      const valid = new Set(rows.map((r) => r.id))
      const next = new Set([...cur].filter((id) => valid.has(id)))
      return next.size === cur.size ? cur : next
    })
  }, [rows])

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

        <div className={`profiles-split${active ? ' open' : ''}`}>
          <section className="card fill" style={{ minHeight: 220 }}>
            {selected.size > 0 ? (
              <div className="toolbar selbar" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <span className="selcount" style={{ fontWeight: 600 }}>{t.profiles.selected(selected.size)}</span>
                <button type="button" className="btn sm danger" disabled={!!busy} onClick={trashSelected}>
                  <Icon name="trash" className="icon sm" />{t.common.delete}
                </button>
                <button type="button" className="btn sm" disabled={!!busy} onClick={() => setSelected(new Set())}>
                  {t.common.cancel}
                </button>
              </div>
            ) : rows.length > 0 && (
              <div className="toolbar">
                <div className="searchfield">
                  <Icon name="search" />
                  <input id="mobile-search" className="input" placeholder={t.mobile.search}
                    value={query} onChange={(e) => setQuery(e.target.value)} aria-label={t.mobile.search} />
                </div>
                {query && (
                  <button type="button" className="btn sm ghost" onClick={() => setQuery('')}>
                    <Icon name="x" className="icon xs" />
                  </button>
                )}
              </div>
            )}
            <div className="tablewrap">
              {rows.length === 0 ? (
                <EmptyState icon="mobile" title={t.mobile.emptyTitle} text={canUse ? t.mobile.emptyText : undefined}>
                  {canUse && <button className="btn primary" onClick={() => setEditing('new')}><Icon name="plus" />{t.mobile.newDevice}</button>}
                </EmptyState>
              ) : visibleRows.length === 0 ? (
                <EmptyState icon="search" title={t.profiles.noMatchTitle} />
              ) : (
                <table className={`table mobile-table${dragging ? ' reordering' : ''}`}>
                  <colgroup>
                    <col style={{ width: 44 }} />
                    <col style={{ width: 54 }} />
                    <col className="mob-col-name" />
                    <col className="mob-col-proxy" />
                    <col className="mob-col-state" />
                    <col className="mob-col-last" />
                    <col className="mob-col-actions" />
                  </colgroup>
                  <thead>
                    <tr>
                      <th className="col-check">
                        <input
                          type="checkbox"
                          checked={allSelected}
                          onChange={toggleAll}
                          aria-label={t.profiles.selectAll(visibleRows.length)}
                        />
                      </th>
                      <th className="col-no">{t.profiles.colNo}</th>
                      <th>{t.mobile.colName}</th>
                      <th>{t.mobile.colProxy}</th>
                      <th>{t.mobile.colState}</th>
                      <th>{t.mobile.colLastOpen}</th>
                      <th className="col-actions" />
                    </tr>
                  </thead>
                  <tbody ref={body}>
                    {visibleRows.map((r, i) => {
                      const working = busy === r.id || busyStates.includes(r.state)
                      const running = r.state === 'running'
                      const isSelected = selected.has(r.id)
                      const isActive = activeId === r.id
                      const isDragging = dragging === r.id
                      const cls = [
                        isSelected ? 'selected' : '',
                        isActive ? 'active' : '',
                        isDragging ? 'dragging' : '',
                      ].filter(Boolean).join(' ')
                      return (
                        <tr
                          key={r.id}
                          data-id={r.id}
                          className={cls || undefined}
                          onPointerDown={(e) => {
                            if (!(e.target as HTMLElement).closest(CONTROLS) || (e.target as HTMLElement).closest('.grip')) {
                              press(e, r.id)
                            }
                          }}
                          onClick={(e) => {
                            if (e.detail <= 1 && !(e.target as HTMLElement).closest(CONTROLS)) {
                              setActiveId((cur) => (cur === r.id ? null : r.id))
                            }
                          }}
                        >
                          <td className="col-check">
                            <input
                              type="checkbox"
                              checked={isSelected}
                              onClick={(e) => toggle(r.id, e.shiftKey)}
                              onChange={() => {}}
                              aria-label={r.name}
                            />
                          </td>
                          <td className="col-no">
                            <span className="serialcell">
                              <button
                                type="button"
                                className="grip"
                                data-grip={r.id}
                                aria-label={t.profiles.dragRow(r.name)}
                                title={t.profiles.dragHint}
                                onKeyDown={(e) => keyMove(e, r.id)}
                              >
                                <Icon name="grip" className="icon xs" />
                              </button>
                              {i + 1}
                              {r.pinned && (
                                <span className="pinmark" title={t.menu.pinned} aria-label={t.menu.pinned}>
                                  <Icon name="pin" className="icon xs" />
                                </span>
                              )}
                            </span>
                          </td>
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
                              <>
                                <button className="iconbtn sm" disabled={working} onClick={() => installAurora(r.id)} aria-label={t.mobile.installAurora} title={t.mobile.installAurora}>
                                  <Icon name="cart" />
                                </button>
                                <button className="iconbtn sm" disabled={working} onClick={() => installApps(r.id)} aria-label={t.mobile.installApps} title={t.mobile.installApps}>
                                  <Icon name="download" />
                                </button>
                              </>
                            )}
                            <button className={`iconbtn sm${running ? '' : ' slot'}`} disabled={!running}
                              onClick={() => run(r.id, () => Api.ShowMobileDevice(r.id))} aria-label={t.mobile.show} title={running ? t.mobile.show : undefined}
                              aria-hidden={!running || undefined} tabIndex={running ? undefined : -1}><Icon name="monitor" /></button>
                            <MobileRowMenu row={r} running={running} busy={working} recording={recording.has(r.id)}
                              onAurora={() => installAurora(r.id)}
                              onView={() => setViewing(r)}
                              onScreenshot={() => screenshot(r.id)} onRecord={() => toggleRecording(r.id)}
                              onTop={() => toggleTop(r.id)} onEdit={() => setActiveId(r.id)}
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
          {active && (
            <MobileDrawer
              key={active.id}
              row={active}
              proxies={proxies}
              knownTags={allTags}
              onClose={() => setActiveId(null)}
              onStart={(id) => open(rows.find((r) => r.id === id)!)}
              onStop={(id) => run(id, () => Api.StopMobileDevice(id))}
              onShow={(id) => run(id, () => Api.ShowMobileDevice(id))}
              onView={(row) => setViewing(row)}
              onScreenshot={screenshot}
              onRecord={toggleRecording}
              onTop={toggleTop}
              onBackup={(id) => run(id, () => Api.BackupMobileDevice(id), t.mobile.backupDone)}
              onRestore={async (id) => {
                const r = rows.find((x) => x.id === id)
                if (!r) return
                if (await confirm({ ...t.confirm.restoreMobile(relativeTime(r.backedUpAt)), danger: true })) {
                  run(id, () => Api.RestoreMobileBackup(id), t.mobile.restoreDone)
                }
              }}
              onInstallAurora={installAurora}
              onInstallApps={installApps}
              onTrash={trash}
              recording={recording.has(active.id)}
              busy={busy === active.id}
              onChanged={reload}
            />
          )}
        </div>
        {rows.length > 0 && canUse && (
          <footer className="mobile-foot">
            {avail?.driver === 'vm' && (
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
            <section className="mobile-help" aria-label={t.mobile.apkHelp}>
              <strong>{t.mobile.apkHelp}:</strong> {t.mobile.apkHint} {avail?.driver === 'vm' && t.mobile.apkWhere}{' · '}
              <a href="https://f-droid.org/packages/com.aurora.store/" onClick={(e) => { e.preventDefault(); openExternal('https://f-droid.org/packages/com.aurora.store/') }}>Aurora Store</a>{' · '}
              <a href="https://f-droid.org/F-Droid.apk" onClick={(e) => { e.preventDefault(); openExternal('https://f-droid.org/F-Droid.apk') }}>F-Droid</a>{' · '}
              <a href={MOBILE_APPS_GUIDE} onClick={(e) => { e.preventDefault(); openExternal(MOBILE_APPS_GUIDE) }}>{t.mobile.apkGuide}</a>
            </section>
          </footer>
        )}
      </div>
      {viewing && <MobileViewer row={viewing} onClose={() => setViewing(null)} />}
      {editing === 'new' && (
        <MobileDialog row={null} onClose={() => setEditing(null)}
          onSaved={(name) => { setEditing(null); reload(); toast.success(t.mobile.created(name)) }} />
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
