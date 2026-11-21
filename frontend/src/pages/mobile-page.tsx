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
import { MobileDrawer } from '../components/mobile-drawer'
import { MobileGpsDialog } from '../components/mobile-gps-dialog'
import { MobileMacroPanel } from '../components/mobile-macro-panel'
import { MobileSyncDialog } from '../components/mobile-sync-dialog'
import { MobileViewer } from '../components/mobile-viewer'
import { PageHead } from '../components/page-head'
import { useToast } from '../components/toast'
import { useConfirm } from '../components/confirm'
import { useRowDrag } from '../components/use-row-drag'
import { PopMenu } from '../components/pop-menu'

type Props = { mobile: ReturnType<typeof useMobile>; onOpenPro: () => void; onOpenSettings: () => void }

const busyStates = ['provisioning', 'starting', 'stopping', 'working']
const installReasons = ['anymobile_runtime_missing', 'anymobile_image_missing']
const installEngines = ['anymobile-avo', 'anymobile-avo-image', 'anymobile-avo-google-play']
const CONTROLS = 'input, button, a, select, textarea, label, [role="menu"], [role="menuitem"]'
const MOBILE_ORDER_KEY = 'anyprofile.mobileOrder'
const MOBILE_VIEW_KEY = 'anyprofile.mobileView'
const mobileViews = ['cards', 'list'] as const
type MobileView = typeof mobileViews[number]

function readMobileView(): MobileView {
  try { return localStorage.getItem(MOBILE_VIEW_KEY) === 'list' ? 'list' : 'cards' } catch { return 'cards' }
}

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
  const [recording, setRecording] = useState<Set<string>>(new Set())
  const [viewing, setViewing] = useState<service.MobileRow | null>(null)
  const [gpsRow, setGpsRow] = useState<service.MobileRow | null>(null)
  const [syncOpen, setSyncOpen] = useState(false)
  const [macroOpen, setMacroOpen] = useState(false)
  const [syncActive, setSyncActive] = useState(false)
  const [order, setOrder] = useState<Map<string, number>>(readMobileOrder)
  const [view, setView] = useState(readMobileView)
  const [draggedCard, setDraggedCard] = useState<service.MobileRow | null>(null)
  const [dropCard, setDropCard] = useState<string | null>(null)
  const changeView = (next: MobileView) => {
    setView(next)
    setDraggedCard(null)
    setDropCard(null)
    try { localStorage.setItem(MOBILE_VIEW_KEY, next) } catch { /* keep the in-memory choice */ }
  }
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

  const collection = useRef<HTMLDivElement>(null)
  const body = useRef<HTMLTableSectionElement>(null)
  const { dragging, press } = useRowDrag(body, visibleRows, reorder)
  const regrip = useRef<string | null>(null)
  useEffect(() => {
    if (!regrip.current) return
    collection.current?.querySelector<HTMLElement>(`[data-grip="${regrip.current}"]`)?.focus()
    regrip.current = null
  })

  const keyMove = (e: React.KeyboardEvent, id: string) => {
    const step = e.key === 'ArrowUp' || (view === 'cards' && e.key === 'ArrowLeft') ? -1
      : e.key === 'ArrowDown' || (view === 'cards' && e.key === 'ArrowRight') ? 1 : 0
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

  const canUse = !!avail?.pro && !!avail?.ready
  const canOpen = (_r: service.MobileRow) => canUse
  // Nothing installed and nothing in the signed list: the runtime is not out yet.
  const unpublished = !!avail?.install && !avail.install.runtime.installed && !avail.install.runtime.latest
  const update = avail?.engines?.find((st) => st.canUpdate) ?? (avail?.install?.canUpdate ? avail.install : undefined)
  const needsEngineUpdate = !!avail?.pro && avail.ready && !!update
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

  const open = (r: Pick<service.MobileRow, 'id'>, justCreated = false) => run(r.id, async () => {
    const res = await Api.StartMobileDevice(r.id).catch((err) => {
      if (justCreated) throw new Error(t.mobile.createdButStopped(errorText(err)))
      throw err
    })
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

  const toggleAudio = (id: string) => run(id, async () => {
    const muted = await Api.MobileWindowMuted(id)
    await Api.SetMobileWindowMuted(id, !muted)
    toast.success(muted ? t.mobile.audioOn : t.mobile.audioOff)
  })

  const pushMedia = (id: string) => run(id, async () => {
    const n = await Api.PushMobileMedia(id)
    if (n > 0) toast.success(t.mobile.mediaPushed(n))
  })

  const openShared = (id: string) => run(id, async () => {
    await Api.OpenMobileSharedFolder(id)
  })

  const arrange = async (layout: string) => {
    try {
      const n = await Api.ArrangeMobileWindows(layout)
      toast.success(t.mobile.arranged(n))
    } catch (e) {
      toast.error(errorText(e))
    }
  }

  const bulkStart = async () => {
    const ids = rows.filter((r) => selected.has(r.id) && canOpen(r)).map((r) => r.id)
    const res = await Api.BulkStartMobileDevices(ids)
    toast.success(t.mobile.bulkDone(res.ok, res.failed))
    setSelected(new Set())
    reload()
  }

  const bulkStop = async () => {
    const ids = [...selected]
    const res = await Api.BulkStopMobileDevices(ids)
    toast.success(t.mobile.bulkDone(res.ok, res.failed))
    setSelected(new Set())
    reload()
  }

  const bulkInstall = async () => {
    const ids = [...selected]
    const res = await Api.BulkInstallMobileApps(ids)
    if (res.ok + res.failed > 0) toast.success(t.mobile.bulkDone(res.ok, res.failed))
  }

  const runningRows = rows.filter((r) => r.state === 'running')

  const installApps = (id: string) => run(id, async () => {
    const n = await Api.InstallMobileApps(id)
    if (n > 0) toast.success(t.mobile.appsInstalled(n))
  })

  const installAurora = (id: string) => run(id, () => Api.InstallMobileAurora(id), t.mobile.auroraInstalled)

  const renderDevice = (r: service.MobileRow, i: number) => {
    const card = view === 'cards'
    const working = busy === r.id || busyStates.includes(r.state)
    const running = r.state === 'running'
    const stopping = running || r.state === 'stopping'
    const isActive = activeId === r.id
    const phase = r.state === 'starting' ? t.mobile.startPhase[mobile.phases?.[r.id] ?? ''] : undefined
    const model = modelOf(r.identityJson)
    const lastOpen = r.lastOpenAt ? relativeTime(r.lastOpenAt) : t.mobile.never
    const proxy = proxyLabel(r)
    const cls = [selected.has(r.id) && 'selected', isActive && 'active', running && 'running',
      (card ? draggedCard?.id === r.id : dragging === r.id) && 'dragging'].filter(Boolean).join(' ')
    const details = () => setActiveId((cur) => (cur === r.id ? null : r.id))
    const onClick = (e: React.MouseEvent) => {
      if (e.detail <= 1 && !(e.target as Element).closest(CONTROLS)) details()
    }
    const checkbox = <input type="checkbox" checked={selected.has(r.id)} aria-label={r.name}
      onClick={(e) => toggle(r.id, e.shiftKey)} onChange={() => {}} />
    const grip = <button type="button" className="grip" data-grip={r.id} aria-label={t.profiles.dragRow(r.name)}
      title={t.profiles.dragHint} onKeyDown={(e) => keyMove(e, r.id)} draggable={card || undefined}
      onDragStart={card ? (e) => {
        e.dataTransfer.setData('text/plain', r.id)
        e.dataTransfer.effectAllowed = 'move'
        setDraggedCard(r)
      } : undefined}
      onDragEnd={card ? () => { setDraggedCard(null); setDropCard(null) } : undefined}>
      <Icon name="grip" className="icon xs" />
    </button>
    const pin = r.pinned && <span className="pinmark" title={t.menu.pinned} aria-label={t.menu.pinned}><Icon name="pin" className="icon xs" /></span>
    const name = <button type="button" className="mobile-name" title={r.name} aria-expanded={isActive} onClick={details}>{r.name}</button>
    const tags = r.tags.map((tag) => <span key={tag} className={card ? 'chip mumu-tag' : 'tag'}>
      {card && <span className="tagdot" style={{ background: `var(--tag-${tagColor(tag)})` }} />}{tag}
    </span>)
    const status = <span role={phase ? 'status' : undefined} className="mobile-device-status">
      <span className={`mobile-state-dot${running ? ' is-running' : working ? ' is-busy' : ''}`} />
      {phase || t.mobile.state[r.state] || r.state}
    </span>
    const error = r.error && <small className="danger mobile-device-error">{r.error}</small>
    const specs = t.mobile.deviceSpecs(r.cpus, r.ramMb / 1024)
    const actions = <>
      <button type="button" className={`iconbtn sm${running ? '' : ' slot'}`} aria-label={t.mobile.show}
        title={running ? t.mobile.show : undefined} disabled={!running || working} aria-hidden={!running || undefined}
        onClick={() => run(r.id, () => Api.ShowMobileDevice(r.id))}><Icon name="monitor" /></button>
      {card && running && <>
        <button type="button" className="iconbtn sm" disabled={working} onClick={() => installAurora(r.id)} aria-label={t.mobile.installAurora} title={t.mobile.installAurora}><Icon name="cart" /></button>
        <button type="button" className="iconbtn sm" disabled={working} onClick={() => installApps(r.id)} aria-label={t.mobile.installApps} title={t.mobile.installApps}><Icon name="download" /></button>
      </>}
      <MobileRowMenu row={r} running={running} busy={working} recording={recording.has(r.id)}
        onAurora={() => installAurora(r.id)} onApps={() => installApps(r.id)}
        onPushMedia={() => pushMedia(r.id)} onSharedFolder={() => openShared(r.id)}
        onGps={() => setGpsRow(r)} onAudio={() => toggleAudio(r.id)} onView={() => setViewing(r)}
        onScreenshot={() => screenshot(r.id)} onRecord={() => toggleRecording(r.id)}
        onTop={() => toggleTop(r.id)} onEdit={() => setActiveId(r.id)}
        onBackup={() => run(r.id, () => Api.BackupMobileDevice(r.id), t.mobile.backupDone)}
        onRestore={async () => { if (await confirm({ ...t.confirm.restoreMobile(relativeTime(r.backedUpAt)), danger: true })) run(r.id, () => Api.RestoreMobileBackup(r.id), t.mobile.restoreDone) }}
        onTrash={() => trash(r)} />
      <button type="button" className={`btn sm actbtn ${stopping ? 'stop' : 'open'}${working ? ' busy' : ''}`}
        aria-busy={working} disabled={working || (!stopping && !canOpen(r))}
        onClick={() => stopping ? run(r.id, () => Api.StopMobileDevice(r.id)) : open(r)}>
        <Icon name={working ? 'refresh' : stopping ? 'stop' : 'play'} className="icon sm" />{stopping ? t.mobile.close : t.mobile.open}
      </button>
    </>
    if (card) {
      const canDrop = draggedCard && draggedCard.id !== r.id && draggedCard.pinned === r.pinned
      return <div key={r.id} role="listitem" data-id={r.id} className={`mumu-card ${cls}${dropCard === r.id ? ' drop-target' : ''}`}
        onClick={onClick} onDragOver={(e) => {
          if (!canDrop) return
          e.preventDefault()
          e.dataTransfer.dropEffect = 'move'
          setDropCard(r.id)
        }} onDragLeave={(e) => {
          if (!(e.relatedTarget instanceof Node) || !e.currentTarget.contains(e.relatedTarget)) setDropCard((cur) => cur === r.id ? null : cur)
        }} onDrop={(e) => {
          if (!canDrop) return
          e.preventDefault()
          reorder(moveTo(visibleRows.map((row) => row.id), draggedCard.id, i))
          setDraggedCard(null)
          setDropCard(null)
        }}>
        <div className="mumu-card-lead">
          <div className="mumu-card-lead-left">{checkbox}{grip}<span className="mumu-card-idx">{i + 1}</span>{pin}</div>
          <span className={`mobile-state-pill${running ? ' is-running' : working ? ' is-busy' : ''}`}>{status}</span>
        </div>
        <div className="mumu-card-info">
          <div className="mumu-card-head">
            {name}
            {model && <span className="mumu-card-model">{model}</span>}
          </div>
          <div className="mumu-card-pills">
            <span className="mumu-spec-pill">{specs}</span>
            <span className="mumu-spec-pill">{r.screenW || 780}×{r.screenH || 1688} · {r.density || 320} DPI</span>
            {r.withGms && <span className="mumu-spec-pill gms">Google Play</span>}
            {tags}
          </div>
          <div className="mumu-card-meta">
            {proxy}
            <span className="mumu-card-time" title={r.lastOpenAt || undefined}>{lastOpen}</span>
          </div>
          {r.backedUpAt && <div className="mumu-card-sub"><span>{t.mobile.backedUp(relativeTime(r.backedUpAt))}</span></div>}
          {error}
        </div>
        <div className="row mobile-actions">{actions}</div>
      </div>
    }
    return <tr key={r.id} data-id={r.id} className={cls} onClick={onClick} aria-current={isActive || undefined}
      onPointerDown={(e) => { if (!(e.target as Element).closest(CONTROLS) || (e.target as Element).closest('.grip')) press(e, r.id) }}>
      <td className="col-check">{checkbox}</td>
      <td className="col-no"><span className="serialcell">{grip}{i + 1}</span></td>
      <td className="cellname">
        <div className="mobile-list-title">{name}{pin}</div>
        <span className="sub"><span>{specs}</span>{model && <span>{model}</span>}{r.withGms && <span>Google Play</span>}{tags}</span>
        <div className="mobile-proxy-inline">{proxy}</div>
        <span className="mobile-state-inline">{status}</span>
        {error}
      </td>
      <td className="mobile-proxy">{proxy}</td>
      <td className="mobile-state">{status}</td>
      <td className="mobile-last" title={r.lastOpenAt || undefined}>{lastOpen}</td>
      <td className="col-actions"><div className="row mobile-actions">{actions}</div></td>
    </tr>
  }

  return (
    <>
      <PageHead title={t.mobile.title} badge={t.mobile.beta} sub={t.mobile.total(rows.length)}>
        {runningRows.length > 0 && (
          <>
            <span className="mobile-deck-pill" role="status">
              <span className="mobile-state-dot is-running" />
              {runningRows.length}/{rows.length} {t.mobile.state.running}
            </span>
            <button className="btn sm nodrag" onClick={() => arrange('row')} title={t.mobile.arrangeRow}>{t.mobile.arrangeRow}</button>
            <button className="btn sm nodrag" onClick={() => arrange('grid2x2')} title={t.mobile.arrangeGrid}>{t.mobile.arrangeGrid}</button>
            <button className={`btn sm nodrag${syncActive ? ' primary' : ''}`} disabled={runningRows.length < 2} onClick={() => setSyncOpen(true)}>
              {syncActive ? t.mobile.syncActive : t.mobile.sync}
            </button>
            <button className="btn sm nodrag" onClick={() => setMacroOpen(true)}>{t.mobile.macro}</button>
          </>
        )}
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
        {needsEngineUpdate && update && (
          <div className="banner info" role="status">
            <Icon name="spark" />
            <div className="grow">
              {installing ? <InstallProgress progress={installing} /> : t.mobile.engineUpdateText(update.runtime.latest)}
            </div>
            {!installing && (
              <button type="button" className="btn sm primary" onClick={install}>
                <Icon name="download" />{t.mobile.engineUpdateBtn}
              </button>
            )}
          </div>
        )}
        {error && <div className="banner danger" role="alert"><Icon name="alert" /><span className="grow">{errorText(error)}</span>
          <button className="iconbtn sm" onClick={() => setError('')} aria-label={t.common.close}><Icon name="x" /></button></div>}

        <div className={`profiles-split mobile-split${active ? ' open' : ''}`}>
          <section className="card fill" style={{ minHeight: 220 }}>
            {rows.length > 0 && <div className={`toolbar mobile-toolbar${selected.size ? ' selecting' : ''}`}>
              {selected.size > 0 ? <div className="row mobile-bulk">
                <span className="selcount">{t.profiles.selected(selected.size)}</span>
                <button type="button" className="btn sm" disabled={!!busy || !rows.some((r) => selected.has(r.id) && canOpen(r))} onClick={bulkStart}>{t.mobile.bulkOpen}</button>
                <button type="button" className="btn sm" disabled={!!busy} onClick={bulkStop}>{t.mobile.bulkClose}</button>
                <button type="button" className="btn sm" disabled={!!busy} onClick={bulkInstall}>{t.mobile.bulkInstall}</button>
                <button type="button" className="btn sm danger" disabled={!!busy} onClick={trashSelected}>
                  <Icon name="trash" className="icon sm" />{t.common.delete}
                </button>
                <button type="button" className="btn sm" disabled={!!busy} onClick={() => setSelected(new Set())}>
                  {t.common.cancel}
                </button>
              </div> : <>
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
              </>}
              <div className="seg mobile-view-switch" role="group" aria-label={t.mobile.viewMode}>
                {mobileViews.map((mode) => <button key={mode} type="button" aria-pressed={view === mode} onClick={() => changeView(mode)}>{t.mobile.views[mode]}</button>)}
              </div>
            </div>}
            <div className="tablewrap" ref={collection}>
              {rows.length === 0 ? (
                <EmptyState icon="mobile" title={t.mobile.emptyTitle} text={canUse ? t.mobile.emptyText : undefined}>
                  {canUse && <button className="btn primary" onClick={() => setEditing('new')}><Icon name="plus" />{t.mobile.newDevice}</button>}
                </EmptyState>
              ) : visibleRows.length === 0 ? (
                <EmptyState icon="search" title={t.profiles.noMatchTitle} />
              ) : view === 'cards' ? <>
                <div className="mumu-listhead">
                  <label className="row checkrow">
                    <input type="checkbox" checked={allSelected} onChange={toggleAll} aria-label={t.profiles.selectAll(visibleRows.length)} />
                    <span className="faint">{t.mobile.total(visibleRows.length)}</span>
                  </label>
                </div>
                <div className="mumu-grid" role="list">{visibleRows.map(renderDevice)}</div>
              </> : (
                <table className={`table mobile-table${dragging ? ' reordering' : ''}`} aria-label={t.mobile.title}>
                  <thead><tr>
                    <th scope="col" className="col-check"><input type="checkbox" checked={allSelected} onChange={toggleAll} aria-label={t.profiles.selectAll(visibleRows.length)} /></th>
                    <th scope="col" className="col-no">{t.profiles.colNo}</th>
                    <th scope="col">{t.mobile.colName}</th>
                    <th scope="col" className="mobile-proxy">{t.mobile.colProxy}</th>
                    <th scope="col" className="mobile-state">{t.mobile.colState}</th>
                    <th scope="col" className="mobile-last">{t.mobile.colLastOpen}</th>
                    <th scope="col" className="col-actions" />
                  </tr></thead>
                  <tbody ref={body}>{visibleRows.map(renderDevice)}</tbody>
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
            <section className="mobile-help" aria-label={t.mobile.apkHelp}>
              <strong>{t.mobile.apkHelp}:</strong> {t.mobile.apkHint} {avail?.driver === 'avo' && t.mobile.apkWhere}{' · '}
              <a href="https://f-droid.org/packages/com.aurora.store/" onClick={(e) => { e.preventDefault(); openExternal('https://f-droid.org/packages/com.aurora.store/') }}>Aurora Store</a>{' · '}
              <a href="https://f-droid.org/F-Droid.apk" onClick={(e) => { e.preventDefault(); openExternal('https://f-droid.org/F-Droid.apk') }}>F-Droid</a>{' · '}
              <a href={MOBILE_APPS_GUIDE} onClick={(e) => { e.preventDefault(); openExternal(MOBILE_APPS_GUIDE) }}>{t.mobile.apkGuide}</a>
            </section>
          </footer>
        )}
      </div>
      {viewing && <MobileViewer row={viewing} onClose={() => setViewing(null)} onOpenGps={() => setGpsRow(viewing)} />}
      {gpsRow && <MobileGpsDialog row={gpsRow} onClose={() => setGpsRow(null)} onToast={(m) => toast.success(m)} />}
      {syncOpen && (
        <MobileSyncDialog running={runningRows} onClose={() => setSyncOpen(false)}
          onChanged={(m) => { toast.success(m); Api.MobileSyncStatus().then((st) => setSyncActive(st.active)).catch(() => {}) }} />
      )}
      {macroOpen && <MobileMacroPanel running={runningRows} onClose={() => setMacroOpen(false)} onToast={(m) => toast.success(m)} />}
      {editing === 'new' && (
        <MobileDialog existingNames={rows.map((r) => r.name)} onClose={() => setEditing(null)}
          onSaved={(id) => { setEditing(null); reload(); void open({ id }, true) }} />
      )}
    </>
  )
}


/** The row's controls stay one action + one overflow menu, like the profile table. */
function MobileRowMenu({ row, running, busy, recording, onAurora, onApps, onPushMedia, onSharedFolder, onGps, onAudio, onView, onScreenshot, onRecord, onTop, onEdit, onBackup, onRestore, onTrash }: {
  row: service.MobileRow; running: boolean; busy: boolean; recording: boolean
  onAurora: () => void; onApps: () => void; onPushMedia: () => void; onSharedFolder: () => void; onGps: () => void; onAudio: () => void; onView: () => void; onScreenshot: () => void; onRecord: () => void; onTop: () => void
  onEdit: () => void; onBackup: () => void; onRestore: () => void; onTrash: () => void
}) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const act = (fn: () => void) => { setOpen(false); fn() }
  return (
    <div className="rowmenu-wrap" ref={ref}>
      <button type="button" className="iconbtn sm" aria-haspopup="menu" aria-expanded={open} aria-label={t.menu.more}
        disabled={busy} onClick={() => setOpen(!open)}><Icon name="more" /></button>
      <PopMenu anchor={ref} open={open} onClose={() => setOpen(false)}>
          {running && <button role="menuitem" onClick={() => act(onAurora)}>{t.mobile.installAurora}</button>}
          {running && <button role="menuitem" onClick={() => act(onApps)}>{t.mobile.installApps}</button>}
          {running && <button role="menuitem" onClick={() => act(onPushMedia)}>{t.mobile.pushMedia}</button>}
          <button role="menuitem" onClick={() => act(onSharedFolder)}>{t.mobile.sharedFolder}</button>
          {running && <button role="menuitem" onClick={() => act(onGps)}>{t.mobile.gps}</button>}
          {running && <button role="menuitem" onClick={() => act(onAudio)}>{t.mobile.toggleAudio}</button>}
          {running && <button role="menuitem" onClick={() => act(onView)}>{t.mobile.viewInApp}</button>}
          {running && <button role="menuitem" onClick={() => act(onScreenshot)}>{t.mobile.screenshot}</button>}
          {running && <button role="menuitem" onClick={() => act(onRecord)}>{recording ? t.mobile.stopRecording : t.mobile.recordScreen}</button>}
          {running && <button role="menuitem" onClick={() => act(onTop)}>{t.mobile.alwaysOnTop}</button>}
          <button role="menuitem" onClick={() => act(onEdit)}>{t.mobile.edit}</button>
          <button role="menuitem" disabled={running} onClick={() => act(onBackup)}>{t.mobile.backup}</button>
          {row.backedUpAt && <button role="menuitem" disabled={running} onClick={() => act(onRestore)}>{t.mobile.restoreBackup}</button>}
          <hr />
          <button role="menuitem" onClick={() => act(onTrash)}>{t.mobile.trash}</button>
      </PopMenu>
    </div>
  )
}

const modelOf = (idJson?: string): string => {
  if (!idJson) return ''
  try {
    const parsed = JSON.parse(idJson) as { model?: string; brand?: string }
    return parsed.model || ''
  } catch {
    return ''
  }
}
