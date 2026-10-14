// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useMemo, useState } from 'react'
import { Api, service } from '../api'
import { language, t } from '../i18n'
import { allTags, errorCode, errorText } from '../lib/format'
import { siteById, type Site } from '../lib/sites'
import { EmptyState } from '../components/empty-state'
import { Icon } from '../components/icons'
import { ProfileEditor } from '../components/profile-editor'
import { ProfileTable, type Sort, type SortKey } from '../components/profile-table'
import { DuplicateDialog } from '../components/duplicate-dialog'
import { BulkProxyDialog, BulkSiteDialog, BulkTagDialog } from '../components/bulk-dialogs'
import { rangeSelect } from '../lib/selection'
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
}

const viewTitle = (v: View, sites: Site[]) =>
  v === 'all' ? t.nav.all : v === 'running' ? t.nav.running : siteById(sites, v.slice(5)).name

export function ProfilesPage({ rows, sites, recent, view, engineReady, fileVaultOn, error, setError, reload, onNew, onNeedEngine, onClearView }: Props) {
  const [query, setQuery] = useState('')
  const [tag, setTag] = useState('')
  const [sort, setSort] = useState<Sort>({ key: 'no', desc: false })
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [editing, setEditing] = useState<service.ProfileRow | null>(null)
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
    const q = query.trim().toLowerCase()
    const out = rows.filter((r) =>
      (view === 'all' || (view === 'running' ? r.state !== 'stopped' : r.site === view.slice(5))) &&
      (!tag || r.tags.includes(tag)) &&
      (!q || r.name.toLowerCase().includes(q) || r.login.toLowerCase().includes(q) || r.tags.some((x) => x.toLowerCase().includes(q)) ||
        siteById(sites, r.site).name.toLowerCase().includes(q)))
    const key = (r: service.ProfileRow) =>
      sort.key === 'name' ? r.name.toLowerCase() : sort.key === 'lastOpen' ? r.lastOpenAt || '' : String(r.serial).padStart(6, '0')
    out.sort((a, b) => key(a).localeCompare(key(b), language) * (sort.desc ? -1 : 1))
    return out
  }, [rows, sites, view, tag, query, sort])

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
  const runningCount = rows.filter((r) => r.state === 'running').length
  const filtered = query || tag || view !== 'all'

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
              <span className="spacer" />
              <button className="btn sm ghost" onClick={() => setSelected(new Set())}>{t.profiles.clearSelection}</button>
            </div>
          ) : (
            <div className="toolbar">
              <div className="searchfield">
                <Icon name="search" />
                <input className="input" placeholder={t.profiles.search} value={query} onChange={(e) => setQuery(e.target.value)} aria-label={t.profiles.search} />
              </div>
              <select className="select" style={{ width: 160 }} value={tag} onChange={(e) => setTag(e.target.value)} aria-label={t.create.tags}>
                <option value="">{t.profiles.allTags}</option>
                {allTags(rows).map((x) => <option key={x} value={x}>{x}</option>)}
              </select>
            </div>
          )}
          <div className="tablewrap">
            {rows.length === 0 ? (
              <EmptyState icon="profiles" title={t.profiles.emptyTitle} text={t.profiles.emptyText}>
                <button className="btn primary" onClick={onNew}><Icon name="plus" />{t.nav.newProfile}</button>
              </EmptyState>
            ) : visible.length === 0 && view === 'running' && !query && !tag ? (
              <EmptyState icon="running" title={t.profiles.runningEmpty} />
            ) : visible.length === 0 ? (
              <EmptyState icon="search" title={t.profiles.noMatchTitle}>
                <button className="btn" onClick={() => { setQuery(''); setTag(''); onClearView() }}>{t.profiles.clearFilters}</button>
              </EmptyState>
            ) : (
              <ProfileTable rows={visible} sites={sites} selected={selected} sort={sort} onSort={onSort}
                onToggle={toggle} onToggleAll={toggleAll}
                onStart={(id) => run(() => Api.StartProfile(id))} onStop={(id) => run(() => Api.StopProfile(id))}
                onFocus={(id) => { Api.FocusProfile(id) }} onEdit={setEditing}
                onDuplicate={setDuplicating} onMessage={toast.success} onError={showError}
                onTrash={(row) => run(async () => { await Api.TrashProfile(row.id); reload() })} />
            )}
          </div>
          <div className="statusbar">
            <span><b>{filtered ? `${visible.length}/${rows.length}` : rows.length}</b> {t.profiles.statusTotal}</span>
            <span><span className={`live${runningCount ? ' on' : ''}`} /><b>{runningCount}</b> {t.profiles.statusRunning}</span>
          </div>
        </section>
      </div>
      {editing && (
        <ProfileEditor row={editing} sites={sites} recent={recent} onClose={() => setEditing(null)} onSaved={() => { setEditing(null); reload() }}
          onTrash={() => run(async () => { await Api.TrashProfile(editing.id); setEditing(null); reload() })} />
      )}
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
