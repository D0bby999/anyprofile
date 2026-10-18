// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useRef, useState, type KeyboardEvent, type PointerEvent } from 'react'
import { service } from '../api'
import { t } from '../i18n'
import { relativeTime, sinceSpan } from '../lib/format'
import { machineLabel } from '../lib/persona'
import type { Site } from '../lib/sites'
import { Icon } from './icons'
import { SiteBadges } from './site-glyph'
import { RowMenu } from './row-menu'
import { HealthPill } from './health-pill'
import type { Column, TablePrefs } from '../lib/table-prefs'
import { groupOf, moveTo } from '../lib/profile-order'

export type SortKey = 'no' | 'name' | 'lastOpen'
export type Sort = { key: SortKey; desc: boolean }

type Props = {
  rows: service.ProfileRow[]
  sites: Site[]
  selected: Set<string>
  sort: Sort
  onSort: (key: SortKey) => void
  onToggle: (id: string, shift?: boolean) => void
  onToggleAll: () => void
  onStart: (id: string) => void
  onStop: (id: string) => void
  onFocus: (id: string) => void
  onEdit: (row: service.ProfileRow) => void
  onDuplicate: (row: service.ProfileRow) => void
  onTrash: (row: service.ProfileRow) => void
  onMessage: (msg: string) => void
  onError: (msg: string) => void
  /** The profile whose details are open, and a click on a row (not on its controls). */
  activeId?: string | null
  /** The row the arrow keys point at. */
  cursorId?: string | null
  onSelectRow?: (id: string) => void
  /** Profiles an AI agent is driving now. */
  agents?: Set<string>
  prefs?: TablePrefs
  /** Set when rows can be dragged into a new order: gets the shown rows' IDs in that order. */
  onReorder?: (ids: string[]) => void
}

const CONTROLS = 'input, button, a, select, textarea, label, [role="menu"]'

/** Last opened, or how long the browser has been running. */
function LastOpened({ row, now }: { row: service.ProfileRow; now: Date }) {
  if (row.state === 'running') {
    return <span className="lastopen running"><span className="live on" />{t.profiles.runningFor(sinceSpan(row.lastOpenAt, now))}</span>
  }
  if (row.state === 'starting') return <span className="lastopen">{t.profiles.opening}…</span>
  if (row.state === 'stopping') return <span className="lastopen">{t.profiles.stopping}…</span>
  return <span className="lastopen muted">{relativeTime(row.lastOpenAt, now)}</span>
}

function ProxyCell({ row }: { row: service.ProfileRow }) {
  if (!row.proxyId) return <span className="cellproxy muted"><Icon name="direct" className="icon sm" />{t.common.direct}</span>
  // The exit measured at the last launch wins over the proxy's last check.
  const ip = row.lastExit?.ip || row.proxyExitIp
  const cc = row.lastExit?.country || row.proxyCountry
  const moved = (row.flags ?? []).some((f) => f.id === 'country_changed')
  return (
    <span className="cellproxy">
      <span className={`cc${moved ? ' danger' : ''}`}>{cc || '?'}</span>
      <span>
        {ip ? <span className="mono selectable">{ip}</span> : <span className="faint">{t.proxies.unchecked}</span>}
        <small title={row.proxyLabel}>{row.proxyLabel}</small>
      </span>
    </span>
  )
}

export function ProfileTable(p: Props) {
  // One clock for the whole table, so running times tick without a timer per row.
  const [now, setNow] = useState(() => new Date())
  useEffect(() => {
    const id = setInterval(() => setNow(new Date()), 60000)
    return () => clearInterval(id)
  }, [])
  const allSelected = p.rows.length > 0 && p.rows.every((r) => p.selected.has(r.id))

  // Dragging a row by its grip: the rows show the new order as the pointer
  // moves; letting go keeps it, Escape puts it back. Pinned rows stay among
  // the pinned, the others among themselves.
  const [drag, setDrag] = useState<{ id: string; ids: string[] } | null>(null)
  const body = useRef<HTMLTableSectionElement>(null)
  const byId = new Map(p.rows.map((r) => [r.id, r]))
  const rows = drag ? drag.ids.map((id) => byId.get(id)).filter((r): r is service.ProfileRow => !!r) : p.rows
  const regrip = useRef<string | null>(null)
  useEffect(() => {
    if (!regrip.current) return
    body.current?.querySelector<HTMLElement>(`[data-grip="${regrip.current}"]`)?.focus()
    regrip.current = null
  })
  const startDrag = (e: PointerEvent, id: string) => {
    if (e.button !== 0 || !p.onReorder) return
    e.preventDefault()
    e.currentTarget.setPointerCapture?.(e.pointerId)
    setDrag({ id, ids: p.rows.map((r) => r.id) })
  }
  const moveDrag = (e: PointerEvent) => {
    if (!drag || !body.current) return
    const trs = [...body.current.querySelectorAll<HTMLElement>('tr[data-id]')]
    const others = trs.filter((tr) => tr.dataset.id !== drag.id)
    const to = others.filter((tr) => { const b = tr.getBoundingClientRect(); return b.top + b.height / 2 < e.clientY }).length
    const [lo, hi] = groupOf(rows, drag.ids.indexOf(drag.id))
    const ids = moveTo(drag.ids, drag.id, to, lo, hi)
    if (ids.some((x, i) => x !== drag.ids[i])) setDrag({ ...drag, ids })
    // Near the edge of the list, it scrolls on.
    const wrap = body.current.closest('.tablewrap')
    const box = wrap?.getBoundingClientRect()
    if (wrap && box) {
      if (e.clientY < box.top + 48) wrap.scrollTop -= 14
      else if (e.clientY > box.bottom - 40) wrap.scrollTop += 14
    }
  }
  const endDrag = (keep: boolean) => {
    if (!drag) return
    if (keep && drag.ids.some((x, i) => x !== p.rows[i]?.id)) p.onReorder?.(drag.ids)
    setDrag(null)
  }
  useEffect(() => {
    if (!drag) return
    const esc = (e: globalThis.KeyboardEvent) => { if (e.key === 'Escape') { e.stopPropagation(); setDrag(null) } }
    window.addEventListener('keydown', esc, true)
    return () => window.removeEventListener('keydown', esc, true)
  }, [drag])
  // With the grip focused, the arrow keys move the row one place.
  const keyMove = (e: KeyboardEvent, id: string) => {
    const step = e.key === 'ArrowUp' ? -1 : e.key === 'ArrowDown' ? 1 : 0
    if (!step || !p.onReorder) return
    e.preventDefault()
    e.stopPropagation()
    const ids = p.rows.map((r) => r.id)
    const i = ids.indexOf(id)
    const [lo, hi] = groupOf(p.rows, i)
    if (i + step < lo || i + step > hi) return
    regrip.current = id
    p.onReorder(moveTo(ids, id, i + step))
  }
  const head = (key: SortKey, label: string, className?: string) => (
    <th className={`sortable ${className ?? ''}`} onClick={() => p.onSort(key)} title={key === 'lastOpen' ? t.profiles.sortLastOpen : undefined}
      aria-sort={p.sort.key === key ? (p.sort.desc ? 'descending' : 'ascending') : undefined}>
      <span className="sorthead">{label}{p.sort.key === key && <Icon name={p.sort.desc ? 'sortDown' : 'sortUp'} className="icon sm" />}</span>
    </th>
  )
  const shown = (c: Column) => !p.prefs?.hidden.includes(c)
  return (
    <table className={`table profiles${p.prefs?.density === 'compact' ? ' compact' : ''}${drag ? ' reordering' : ''}`}>
      <thead>
        <tr>
          <th className="col-check"><input type="checkbox" checked={allSelected} onChange={p.onToggleAll} aria-label={t.profiles.selected(p.rows.length)} /></th>
          {head('no', t.profiles.colNo, 'col-no')}
          {head('name', t.profiles.colName)}
          {shown('site') && <th className="col-site"><span className="sitehead">{t.profiles.colSite}</span></th>}
          {shown('proxy') && <th className="col-proxy">{t.profiles.colProxy}</th>}
          {shown('status') && head('lastOpen', t.profiles.colStatus, 'col-status')}
          <th className="col-actions" />
        </tr>
      </thead>
      <tbody ref={body}>
        {rows.map((r, i) => {
          const running = r.state === 'running'
          const busy = r.state === 'starting' || r.state === 'stopping'
          const agent = !!p.agents?.has(r.id)
          // An agent's row is tinted instead of a chip, so the status cell keeps only the health.
          const cls = [p.selected.has(r.id) ? 'selected' : '', running ? 'running' : '', agent ? 'agent' : '', p.activeId === r.id ? 'active' : '', p.cursorId === r.id ? 'cursor' : '', drag?.id === r.id ? 'dragging' : ''].join(' ').trim()
          return (
            <tr key={r.id} data-id={r.id} className={cls || undefined} aria-current={p.activeId === r.id ? 'true' : undefined} title={agent ? t.agent.title : undefined}
              onClick={(e) => { if (e.detail <= 1 && !(e.target as HTMLElement).closest(CONTROLS)) p.onSelectRow?.(r.id) }}
              onDoubleClick={(e) => { if (!(e.target as HTMLElement).closest(CONTROLS)) (running ? p.onFocus(r.id) : !busy && p.onStart(r.id)) }}>
              <td className="col-check"><input type="checkbox" checked={p.selected.has(r.id)}
                onClick={(e) => p.onToggle(r.id, e.shiftKey)} onChange={() => {}} aria-label={r.name} /></td>
              <td className="col-no">
                {/* The row's place in the list; the fixed #serial is in the drawer and the window title. */}
                <span className="serialcell">
                  {p.onReorder && (
                    <button className="grip" data-grip={r.id} aria-label={t.profiles.dragRow(r.name)} title={t.profiles.dragHint}
                      onPointerDown={(e) => startDrag(e, r.id)} onPointerMove={moveDrag} onPointerUp={() => endDrag(true)}
                      onPointerCancel={() => endDrag(false)} onKeyDown={(e) => keyMove(e, r.id)}>
                      <Icon name="grip" className="icon xs" />
                    </button>
                  )}
                  {i + 1}{r.pinned && <span className="pinmark" title={t.menu.pinned} aria-label={t.menu.pinned}><Icon name="pin" className="icon xs" /></span>}</span>
              </td>
              <td className="cellname">
                <b className="selectable">{r.name}</b>
                <span className="sub">
                  <span>{machineLabel(r)}</span>
                  {r.engine === 'fury' && <span className="chip engine-tag">{t.engines.names.fury}</span>}
                  {r.tags.map((x) => <span key={x} className="tag">{x}</span>)}
                </span>
              </td>
              {shown('site') && <td className="col-site"><SiteBadges ids={r.sites} sites={p.sites} /></td>}
              {shown('proxy') && <td className="col-proxy"><ProxyCell row={r} /></td>}
              {shown('status') && (
                <td className="col-status">
                  <span className="cellstatus">
                    <HealthPill row={r} now={now} />
                    {agent && <span className="sr-only">{t.agent.title}</span>}
                    <LastOpened row={r} now={now} />
                  </span>
                </td>
              )}
              <td className="col-actions">
                <span className="row">
                  {/* Always there, hidden while closed: the column keeps its width when a profile opens. */}
                  <button className={`iconbtn sm${running ? '' : ' slot'}`} onClick={() => p.onFocus(r.id)} aria-label={t.profiles.focus}
                    title={running ? t.profiles.focus : undefined} disabled={!running} aria-hidden={!running || undefined} tabIndex={running ? undefined : -1}>
                    <Icon name="focus" />
                  </button>
                  <RowMenu row={r} sites={p.sites} onEdit={() => p.onEdit(r)} onDuplicate={() => p.onDuplicate(r)}
                    onTrash={() => p.onTrash(r)} onMessage={p.onMessage} onError={p.onError} />
                  {running || r.state === 'stopping' ? (
                    <button className="btn sm stop actbtn" disabled={busy} onClick={() => p.onStop(r.id)}>
                      <Icon name="stop" className="icon sm" /><ActLabel shown={busy ? t.profiles.stopping : t.profiles.stop} />
                    </button>
                  ) : (
                    <button className={`btn sm primary actbtn${busy ? ' busy' : ''}`} aria-busy={busy} disabled={busy} onClick={() => p.onStart(r.id)}>
                      <Icon name={busy ? 'refresh' : 'play'} className="icon sm" /><ActLabel shown={busy ? t.profiles.opening : t.profiles.open} />
                    </button>
                  )}
                </span>
              </td>
            </tr>
          )
        })}
      </tbody>
    </table>
  )
}

/** The Open/Close button's label, as wide as its longest one in this language, so the button never resizes. */
function ActLabel({ shown }: { shown: string }) {
  const all = [t.profiles.open, t.profiles.opening, t.profiles.stop, t.profiles.stopping]
  return (
    <span className="actlabel">
      {all.map((x) => <span key={x} aria-hidden={x === shown ? undefined : true} className={x === shown ? undefined : 'off'}>{x}</span>)}
    </span>
  )
}
