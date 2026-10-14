// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'react'
import { service } from '../api'
import { t } from '../i18n'
import { relativeTime, sinceSpan } from '../lib/format'
import { machineLabel } from '../lib/persona'
import { siteById, type Site } from '../lib/sites'
import { Icon } from './icons'
import { SiteGlyph } from './site-glyph'
import { RowMenu } from './row-menu'
import { HealthPill } from './health-pill'
import type { TablePrefs } from '../lib/table-prefs'

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
  prefs?: TablePrefs
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
  const head = (key: SortKey, label: string, className?: string) => (
    <th className={`sortable ${className ?? ''}`} onClick={() => p.onSort(key)}
      aria-sort={p.sort.key === key ? (p.sort.desc ? 'descending' : 'ascending') : undefined}>
      <span className="sorthead">{label}{p.sort.key === key && <Icon name={p.sort.desc ? 'sortDown' : 'sortUp'} className="icon sm" />}</span>
    </th>
  )
  const shown = (c: 'site' | 'proxy' | 'health' | 'lastOpen') => !p.prefs?.hidden.includes(c)
  return (
    <table className={`table${p.prefs?.density === 'compact' ? ' compact' : ''}`}>
      <thead>
        <tr>
          <th className="col-check"><input type="checkbox" checked={allSelected} onChange={p.onToggleAll} aria-label={t.profiles.selected(p.rows.length)} /></th>
          {head('no', t.profiles.colNo, 'col-no')}
          {head('name', t.profiles.colName)}
          {shown('site') && <th className="col-site"><span className="sitehead">{t.profiles.colSite}</span></th>}
          {shown('proxy') && <th className="col-proxy">{t.profiles.colProxy}</th>}
          {shown('health') && <th className="col-health">{t.profiles.colHealth}</th>}
          {shown('lastOpen') && head('lastOpen', t.profiles.colLastOpen, 'col-lastopen')}
          <th className="col-actions" />
        </tr>
      </thead>
      <tbody>
        {p.rows.map((r, i) => {
          const running = r.state === 'running'
          const busy = r.state === 'starting' || r.state === 'stopping'
          const s = siteById(p.sites, r.site)
          const cls = [p.selected.has(r.id) ? 'selected' : '', running ? 'running' : '', p.activeId === r.id ? 'active' : '', p.cursorId === r.id ? 'cursor' : ''].join(' ').trim()
          return (
            <tr key={r.id} className={cls || undefined} aria-current={p.activeId === r.id ? 'true' : undefined}
              onClick={(e) => { if (e.detail <= 1 && !(e.target as HTMLElement).closest(CONTROLS)) p.onSelectRow?.(r.id) }}
              onDoubleClick={(e) => { if (!(e.target as HTMLElement).closest(CONTROLS)) (running ? p.onFocus(r.id) : !busy && p.onStart(r.id)) }}>
              <td className="col-check"><input type="checkbox" checked={p.selected.has(r.id)}
                onClick={(e) => p.onToggle(r.id, e.shiftKey)} onChange={() => {}} aria-label={r.name} /></td>
              <td className="col-no">
                {/* The row's place in the list; the fixed #serial is in the drawer and the window title. */}
                <span className="serialcell">{i + 1}{r.pinned && <span className="pinmark" title={t.menu.pinned} aria-label={t.menu.pinned}><Icon name="pin" className="icon xs" /></span>}</span>
              </td>
              <td className="cellname">
                <b className="selectable">{r.name}</b>
                <span className="sub">
                  <span>{machineLabel(r)}</span>
                  {r.engine === 'fury' && <span className="chip engine-tag">{t.engines.names.fury}</span>}
                  {r.tags.map((x) => <span key={x} className="tag">{x}</span>)}
                </span>
              </td>
              {shown('site') && <td className="col-site"><span className="platform" title={s.name}><SiteGlyph site={s} /><span className="plabel">{s.name}</span></span></td>}
              {shown('proxy') && <td className="col-proxy"><ProxyCell row={r} /></td>}
              {shown('health') && <td className="col-health"><HealthPill row={r} now={now} /></td>}
              {shown('lastOpen') && <td className="nowrap col-lastopen"><LastOpened row={r} now={now} /></td>}
              <td className="col-actions">
                <span className="row">
                  {running && (
                    <button className="iconbtn sm" onClick={() => p.onFocus(r.id)} aria-label={t.profiles.focus} title={t.profiles.focus}>
                      <Icon name="focus" />
                    </button>
                  )}
                  <RowMenu row={r} sites={p.sites} onEdit={() => p.onEdit(r)} onDuplicate={() => p.onDuplicate(r)}
                    onTrash={() => p.onTrash(r)} onMessage={p.onMessage} onError={p.onError} />
                  {running || r.state === 'stopping' ? (
                    <button className="btn sm stop actbtn" disabled={busy} onClick={() => p.onStop(r.id)}>
                      <Icon name="stop" className="icon sm" />{busy ? t.profiles.stopping : t.profiles.stop}
                    </button>
                  ) : (
                    <button className={`btn sm primary actbtn${busy ? ' busy' : ''}`} aria-busy={busy} disabled={busy} onClick={() => p.onStart(r.id)}>
                      <Icon name={busy ? 'refresh' : 'play'} className="icon sm" />{busy ? t.profiles.opening : t.profiles.open}
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
