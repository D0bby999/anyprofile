// SPDX-License-Identifier: AGPL-3.0-or-later
import { service } from '../api'
import { t } from '../i18n'
import { relativeTime } from '../lib/format'
import { personaLabel } from '../lib/persona'
import { siteById, type Site } from '../lib/sites'
import { Icon } from './icons'
import { SiteGlyph } from './site-glyph'
import { RowMenu } from './row-menu'

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
}

const pillClass: Record<string, string> = { running: 'ok', starting: 'warn', stopping: 'warn' }

function StatePill({ row }: { row: service.ProfileRow }) {
  if (row.state === 'stopped') return <span className="muted">{t.state.stopped}</span>
  return (
    <span className="row">
      <span className={`chip ${pillClass[row.state] ?? ''}`}><span className="dot" />{t.state[row.state as keyof typeof t.state] ?? row.state}</span>
      {row.cdpPort > 0 && <span className="chip warn" title={t.editor.cdpWarning}>{t.profiles.cdp(row.cdpPort)}</span>}
    </span>
  )
}

function ProxyCell({ row }: { row: service.ProfileRow }) {
  if (!row.proxyId) return <span className="cellproxy muted"><Icon name="direct" className="icon sm" />{t.common.direct}</span>
  return (
    <span className="cellproxy">
      <span className="cc">{row.proxyCountry || '?'}</span>
      <span>
        {row.proxyExitIp ? <span className="mono selectable">{row.proxyExitIp}</span> : <span className="faint">{t.proxies.unchecked}</span>}
        <small>{row.proxyLabel}</small>
        {row.proxyExpiry === 'soon' && <span className="chip warn"><span className="dot" />{t.proxies.expiringSoon}</span>}
        {row.proxyExpiry === 'expired' && <span className="chip danger"><span className="dot" />{t.proxies.expired}</span>}
      </span>
    </span>
  )
}

export function ProfileTable(p: Props) {
  const allSelected = p.rows.length > 0 && p.rows.every((r) => p.selected.has(r.id))
  const head = (key: SortKey, label: string, className?: string) => (
    <th className={`sortable ${className ?? ''}`} onClick={() => p.onSort(key)}
      aria-sort={p.sort.key === key ? (p.sort.desc ? 'descending' : 'ascending') : undefined}>
      <span className="sorthead">{label}{p.sort.key === key && <Icon name={p.sort.desc ? 'sortDown' : 'sortUp'} className="icon sm" />}</span>
    </th>
  )
  return (
    <table className="table">
      <thead>
        <tr>
          <th className="col-check"><input type="checkbox" checked={allSelected} onChange={p.onToggleAll} aria-label={t.profiles.selected(p.rows.length)} /></th>
          {head('no', t.profiles.colNo, 'col-no')}
          {head('name', t.profiles.colName)}
          <th>{t.profiles.colSite}</th>
          <th>{t.profiles.colProxy}</th>
          <th>{t.profiles.colState}</th>
          {head('lastOpen', t.profiles.colLastOpen, 'col-lastopen')}
          <th className="col-actions" />
        </tr>
      </thead>
      <tbody>
        {p.rows.map((r) => {
          const running = r.state === 'running'
          const busy = r.state === 'starting' || r.state === 'stopping'
          const s = siteById(p.sites, r.site)
          const cls = [p.selected.has(r.id) ? 'selected' : '', running ? 'running' : ''].join(' ').trim()
          return (
            <tr key={r.id} className={cls || undefined} onDoubleClick={() => (running ? p.onFocus(r.id) : !busy && p.onStart(r.id))}>
              <td className="col-check"><input type="checkbox" checked={p.selected.has(r.id)}
                onClick={(e) => p.onToggle(r.id, e.shiftKey)} onChange={() => {}} aria-label={r.name} /></td>
              <td className="col-no">{r.serial}</td>
              <td className="cellname">
                <b className="selectable">{r.name}</b>
                <span className="sub">
                  <span>{personaLabel(r.personaId)}</span>
                  {r.tags.map((x) => <span key={x} className="tag">{x}</span>)}
                </span>
              </td>
              <td><span className="platform" title={s.name}><SiteGlyph site={s} /><span className="plabel">{s.name}</span></span></td>
              <td><ProxyCell row={r} /></td>
              <td><StatePill row={r} /></td>
              <td className="muted nowrap col-lastopen">{relativeTime(r.lastOpenAt)}</td>
              <td className="col-actions">
                <span className="row">
                  {running && (
                    <button className="iconbtn sm" onClick={() => p.onFocus(r.id)} aria-label={t.profiles.focus} title={t.profiles.focus}>
                      <Icon name="focus" />
                    </button>
                  )}
                  <RowMenu row={r} onEdit={() => p.onEdit(r)} onDuplicate={() => p.onDuplicate(r)}
                    onTrash={() => p.onTrash(r)} onMessage={p.onMessage} onError={p.onError} />
                  {running || r.state === 'stopping' ? (
                    <button className="btn sm stop actbtn" disabled={busy} onClick={() => p.onStop(r.id)}>
                      <Icon name="stop" className="icon sm" />{busy ? t.profiles.stopping : t.profiles.stop}
                    </button>
                  ) : (
                    <button className={`btn sm primary actbtn${busy ? ' busy' : ''}`} disabled={busy} onClick={() => p.onStart(r.id)}>
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
