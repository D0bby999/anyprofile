// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useMemo, useRef, useState } from 'react'
import { Api, service } from '../api'
import { language, t } from '../i18n'
import { allTags, errorCode, errorText, fold } from '../lib/format'
import { siteById, type Site } from '../lib/sites'
import { EmptyState } from '../components/empty-state'
import { Icon } from '../components/icons'
import { ProfileDrawer } from '../components/profile-drawer'
import { ProfileTable, type Sort, type SortKey } from '../components/profile-table'
import { DuplicateDialog } from '../components/duplicate-dialog'
import { BulkProxyDialog, BulkSiteDialog, BulkTagDialog } from '../components/bulk-dialogs'
import { rangeSelect } from '../lib/selection'
import { readTablePrefs, writeTablePrefs, type TablePrefs } from '../lib/table-prefs'
import { ViewMenu } from '../components/view-menu'
import { registerShortcuts } from '../lib/shortcuts'
import { needsAttention } from '../lib/health'
import type { View } from '../components/sidebar'
import { PageHead } from '../components/page-head'
import { useToast } from '../components/toast'

type Props = {
  rows: service.ProfileRow[]
  sites: Site[]
  recent: string[]
  view: View
  engineReady: boolean
  fileVaultOn: boolean
  error: string
  setError: (e: string) => void
  reload: () => void
  onNew: () => void
  onNeedEngine: () => void
  onClearView: () => void
  /** The profile whose details drawer is open; App owns it so ⌘K can open one. */
  activeId: string | null
  setActiveId: (id: string | null | ((cur: string | null) => string | null)) => void
}

const viewTitle = (v: View, sites: Site[]) =>
  v === 'all' ? t.nav.all : v === 'running' ? t.nav.running : v === 'attention' ? t.nav.attention
    : v.startsWith('tag:') ? v.slice(4) : siteById(sites, v.slice(5)).name

function inView(r: service.ProfileRow, v: View): boolean {
  if (v === 'all') return true
  if (v === 'running') return r.state !== 'stopped'
  if (v === 'attention') return needsAttention(r)
  if (v.startsWith('tag:')) return r.tags.includes(v.slice(4))
  return r.site === v.slice(5)
}

export function ProfilesPage({ rows, sites, recent, view, engineReady, fileVaultOn, error, setError, reload, onNew, onNeedEngine, onClearView, activeId, setActiveId }: Props) {
  const [query, setQuery] = useState('')
  const [prefs, setPrefs] = useState<TablePrefs>(readTablePrefs)
  const changePrefs = (p: TablePrefs) => { setPrefs(p); writeTablePrefs(p) }
  const [sort, setSort] = useState<Sort>({ key: 'no', desc: false })
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [duplicating, setDuplicating] = useState<service.ProfileRow | null>(null)
  const [bulkDialog, setBulkDialog] = useState<'site' | 'tag' | 'proxy' | null>(null)
  const [lastToggled, setLastToggled] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const toast = useToast()

  const showError = (e: unknown) => {
    const action = errorCode(e) === 'engine_missing' ? { label: t.profiles.installEngine, run: onNeedEngine } : undefined
    toast.error(errorText(e), action)
  }
  // Load and launch failures arrive through useProfiles; report each once.
  useEffect(() => {
    if (!error) return
    showError(error)
    setError('')
  }, [error])

  const visible = useMemo(() => {
    const q = fold(query.trim())
    const serial = /^#(\d+)$/.exec(q)?.[1]
    const matches = (r: service.ProfileRow) => serial ? String(r.serial) === serial
      : [r.name, r.login, r.notes, r.proxyExitIp, siteById(sites, r.site).name, ...r.tags].some((x) => fold(x ?? '').includes(q))
    const out = rows.filter((r) => inView(r, view) && (!q || matches(r)))
    const key = (r: service.ProfileRow) =>
      sort.key === 'name' ? r.name.toLowerCase() : sort.key === 'lastOpen' ? r.lastOpenAt || '' : String(r.serial).padStart(6, '0')
    // Pinned profiles stay on top whatever the sort.
    out.sort((a, b) => Number(b.pinned) - Number(a.pinned) || key(a).localeCompare(key(b), language) * (sort.desc ? -1 : 1))
    return out
  }, [rows, sites, view, query, sort])

  const run = async (fn: () => Promise<unknown>) => {
    setBusy(true)
    try {
      await fn()
    } catch (e) {
      showError(e)
    } finally {
      setBusy(false)
    }
  }
  const bulk = (fn: (ids: string[]) => Promise<Record<string, string>>) => run(async () => {
    const errs = Object.values((await fn([...selected])) ?? {})
    if (errs.length) showError(errs[0])
  })
  const onSort = (key: SortKey) => setSort((s) => ({ key, desc: s.key === key ? !s.desc : key === 'lastOpen' }))
  const toggle = (id: string, shift = false) => {
    setSelected((cur) => {
      if (shift && lastToggled) {
        return rangeSelect(visible.map((r) => r.id), lastToggled, id, cur)
      }
      const next = new Set(cur)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
    setLastToggled(id)
  }
  const toggleAll = () => setSelected((cur) => (visible.every((r) => cur.has(r.id)) ? new Set() : new Set(visible.map((r) => r.id))))
  const active = activeId ? rows.find((r) => r.id === activeId) ?? null : null
  // A profile that left the list (trashed, filtered away elsewhere) closes the drawer.
  useEffect(() => { if (activeId && !rows.some((r) => r.id === activeId)) setActiveId(null) }, [rows, activeId])
  const select = (id: string) => setActiveId((cur) => (cur === id ? null : id))

  // Keyboard: arrows move a cursor row (and the drawer with it), Enter opens
  // the details, ⌘Enter the browser, Space selects, ⌘A selects all, Escape
  // closes the drawer, then clears the selection.
  const [cursorId, setCursorId] = useState<string | null>(null)
  const cursor = activeId ?? cursorId
  const keys = useRef({ visible, cursor, activeId, selected })
  keys.current = { visible, cursor, activeId, selected }
  useEffect(() => registerShortcuts([
    { combo: 'down', run: () => move(1) },
    { combo: 'up', run: () => move(-1) },
    { combo: 'enter', when: () => !!keys.current.cursor, run: () => setActiveId(keys.current.cursor) },
    { combo: 'mod+enter', when: () => !!keys.current.cursor, run: () => launch(keys.current.cursor!) },
    { combo: 'space', when: () => !!keys.current.cursor, run: () => toggle(keys.current.cursor!) },
    { combo: 'mod+a', run: () => setSelected(new Set(keys.current.visible.map((r) => r.id))) },
    { combo: 'escape', global: true, when: () => !!keys.current.activeId || keys.current.selected.size > 0, run: (e) => {
      if ((e.target as Element | null)?.closest?.('input, textarea, select')) return
      if (keys.current.activeId) setActiveId(null)
      else setSelected(new Set())
    } },
  ]), []) // the handlers read keys.current, so one registration is enough
  const move = (step: number) => {
    const { visible: v, cursor: c, activeId: open } = keys.current
    const i = c ? v.findIndex((r) => r.id === c) : -1
    const next = v[i < 0 ? (step > 0 ? 0 : v.length - 1) : i + step]
    if (!next) return
    if (open) setActiveId(next.id)
    setCursorId(next.id)
  }
  const launch = (id: string) => {
    const row = keys.current.visible.find((r) => r.id === id)
    if (!row) return
    if (row.state === 'running') Api.FocusProfile(id)
    else if (row.state === 'stopped') run(() => Api.StartProfile(id))
  }

  const runningCount = rows.filter((r) => r.state === 'running').length
  const filtered = query || view !== 'all'
  const filterLabel = view === 'all' ? '' : view === 'running' ? t.filters.running : view === 'attention' ? t.filters.attention
    : view.startsWith('tag:') ? t.filters.tag(view.slice(4)) : t.filters.site(siteById(sites, view.slice(5)).name)

  return (
    <>
      <PageHead title={viewTitle(view, sites)} sub={t.profiles.total(visible.length)}>
        <button className="iconbtn nodrag" onClick={reload} aria-label={t.common.refresh} title={t.common.refresh}><Icon name="refresh" /></button>
      </PageHead>
      <div className="pagebody">
        {!engineReady && (
          <div className="banner warn"><Icon name="engine" /><span className="grow">{t.profiles.engineMissing}</span>
            <button className="btn sm" onClick={onNeedEngine}>{t.profiles.installEngine}</button></div>
        )}
        {!fileVaultOn && <div className="banner warn"><Icon name="shield" /><span className="grow">{t.settings.fileVaultOff}</span></div>}
        <div className={`profiles-split${active ? ' open' : ''}`}>
          <section className="card fill">
            {selected.size > 0 ? (
              <div className="toolbar selecting">
                <span className="selcount">{t.profiles.selected(selected.size)}</span>
                <button className="btn sm primary" disabled={busy} onClick={() => bulk(Api.StartProfiles)}><Icon name="play" className="icon sm" />{t.profiles.openSelected}</button>
                <button className="btn sm" disabled={busy} onClick={() => bulk(Api.StopProfiles)}><Icon name="stop" className="icon sm" />{t.profiles.stopSelected}</button>
                <button className="btn sm danger" disabled={busy}
                  onClick={() => run(async () => { for (const id of selected) await Api.TrashProfile(id); setSelected(new Set()); reload() })}>
                  <Icon name="trash" className="icon sm" />{t.profiles.trashSelected}
                </button>
                <button className="btn sm" disabled={busy} onClick={() => setBulkDialog('site')}><Icon name="pencil" className="icon sm" />{t.bulk.site}</button>
                <button className="btn sm" disabled={busy} onClick={() => setBulkDialog('tag')}><Icon name="plus" className="icon sm" />{t.bulk.tag}</button>
                <button className="btn sm" disabled={busy} onClick={() => setBulkDialog('proxy')}><Icon name="proxy" className="icon sm" />{t.bulk.proxy}</button>
                <button className="btn sm" disabled={busy} onClick={() => run(() => Api.SetPinnedMany([...selected], true))}><Icon name="pin" className="icon sm" />{t.profiles.pinSelected}</button>
                <button className="btn sm" disabled={busy} onClick={() => run(() => Api.SetPinnedMany([...selected], false))}>{t.profiles.unpinSelected}</button>
                <span className="spacer" />
                <button className="btn sm ghost" onClick={() => setSelected(new Set())}>{t.profiles.clearSelection}</button>
              </div>
            ) : (
              <div className="toolbar">
                <div className="searchfield">
                  <Icon name="search" />
                  <input id="profile-search" className="input" placeholder={t.profiles.search} value={query} onChange={(e) => setQuery(e.target.value)} aria-label={t.profiles.search} />
                </div>
                {filterLabel && (
                  <span className="filterchips">
                    <span className="chip accent filterchip">{filterLabel}
                      <button type="button" onClick={onClearView} aria-label={t.filters.remove(filterLabel)}><Icon name="x" className="icon xs" /></button>
                    </span>
                  </span>
                )}
                <span className="spacer" />
                <ViewMenu prefs={prefs} onChange={changePrefs} />
              </div>
            )}
            <div className="tablewrap">
              {rows.length === 0 ? (
                <EmptyState icon="profiles" title={t.profiles.emptyTitle} text={t.profiles.emptyText}>
                  <button className="btn primary" onClick={onNew}><Icon name="plus" />{t.nav.newProfile}</button>
                </EmptyState>
              ) : visible.length === 0 && view === 'running' && !query ? (
                <EmptyState icon="running" title={t.profiles.runningEmpty} />
              ) : visible.length === 0 ? (
                <EmptyState icon="search" title={t.profiles.noMatchTitle}>
                  <button className="btn" onClick={() => { setQuery(''); onClearView() }}>{t.profiles.clearFilters}</button>
                </EmptyState>
              ) : (
                <ProfileTable rows={visible} sites={sites} selected={selected} sort={sort} onSort={onSort}
                  onToggle={toggle} onToggleAll={toggleAll}
                  onStart={(id) => run(() => Api.StartProfile(id))} onStop={(id) => run(() => Api.StopProfile(id))}
                  onFocus={(id) => { Api.FocusProfile(id) }} onEdit={(row) => setActiveId(row.id)}
                activeId={activeId} cursorId={cursor} onSelectRow={(id) => { setCursorId(id); select(id) }} prefs={prefs}
                  onDuplicate={setDuplicating} onMessage={toast.success} onError={showError}
                  onTrash={(row) => run(async () => { await Api.TrashProfile(row.id); reload() })} />
              )}
            </div>
            <div className="statusbar">
              <span><b>{filtered ? `${visible.length}/${rows.length}` : rows.length}</b> {t.profiles.statusTotal}</span>
              <span><span className={`live${runningCount ? ' on' : ''}`} /><b>{runningCount}</b> {t.profiles.statusRunning}</span>
            </div>
          </section>
          {active && (
            <ProfileDrawer key={active.id} row={active} sites={sites} recent={recent} knownTags={allTags(rows)}
              onClose={() => setActiveId(null)} onStart={(id) => run(() => Api.StartProfile(id))} onStop={(id) => run(() => Api.StopProfile(id))} />
          )}
        </div>
      </div>
      {duplicating && (
        <DuplicateDialog row={duplicating} onClose={() => setDuplicating(null)}
          onDone={() => { setDuplicating(null); reload() }} />
      )}
      {bulkDialog === 'site' && (
        <BulkSiteDialog ids={[...selected]} sites={sites} recent={recent} onClose={() => setBulkDialog(null)}
          onDone={() => { setBulkDialog(null); setSelected(new Set()); reload() }} />
      )}
      {bulkDialog === 'tag' && (
        <BulkTagDialog ids={[...selected]} onClose={() => setBulkDialog(null)}
          onDone={() => { setBulkDialog(null); setSelected(new Set()); reload() }} />
      )}
      {bulkDialog === 'proxy' && (
        <BulkProxyDialog ids={[...selected]} onClose={() => setBulkDialog(null)}
          onDone={() => { setBulkDialog(null); setSelected(new Set()); reload() }} />
      )}
    </>
  )
}
