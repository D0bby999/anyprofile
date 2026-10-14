// SPDX-License-Identifier: AGPL-3.0-or-later
import { useMemo, useRef, useState } from 'react'
import { t } from '../i18n'
import { siteById, type Site } from '../lib/sites'
import { Dialog } from './dialog'
import { Icon } from './icons'
import { SiteGlyph } from './site-glyph'

const groupOrder = ['video', 'social', 'messaging', 'commerce', 'ads', 'freelance', 'google', 'payments', 'custom', 'other']

type PanelProps = {
  sites: Site[]
  recent: string[]
  isOn: (id: string) => boolean
  /** Returns true when the panel should close. */
  onPick: (id: string) => boolean
  onClose: () => void
  /** Enter on an empty search; without it Enter picks the first platform. */
  onEnterEmpty?: () => void
  className?: string
}

/** The searchable, grouped list of platforms. */
function SitePanel({ sites, recent, isOn, onPick, onClose, onEnterEmpty, className = 'sitepanel' }: PanelProps) {
  const [query, setQuery] = useState('')
  const q = query.trim().toLowerCase()
  const visible = useMemo(() => {
    if (q) return sites.filter((s) => s.name.toLowerCase().includes(q) || (s.domains ?? []).some((d) => d.includes(q)))
    return sites
  }, [sites, q])

  const sections = useMemo(() => {
    if (q) return [{ title: '', items: visible }]
    const out: { title: string; items: Site[] }[] = []
    const recentItems = recent.filter((id) => visible.some((s) => s.id === id)).slice(0, 6).map((id) => siteById(sites, id))
    if (recentItems.length) out.push({ title: t.siteGroups.recent, items: recentItems })
    for (const g of groupOrder) {
      const items = visible.filter((s) => s.group === g)
      if (items.length) out.push({ title: t.siteGroups[g] ?? g, items })
    }
    return out
  }, [q, visible, recent, sites])

  const flat = sections.flatMap((s) => s.items)

  const pick = (id: string) => {
    setQuery('')
    if (onPick(id)) onClose()
  }

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && (e.target as Element).tagName === 'INPUT') {
      e.preventDefault()
      if (!q && onEnterEmpty) onEnterEmpty()
      else if (flat.length) pick(flat[0].id)
    } else if (e.key === 'Escape') {
      e.stopPropagation() // close the panel, not the dialog around it
      onClose()
    }
  }

  return (
    <div className={className} onKeyDown={onKeyDown}>
      <input className="input" autoFocus placeholder={t.sites.search}
        value={query} onChange={(e) => setQuery(e.target.value)} aria-label={t.sites.search} />
      <div className="sitelist">
        {flat.length === 0 && <div className="hint muted" style={{ padding: '8px 4px' }}>{t.sites.none}</div>}
        {sections.map((sec, i) => (
          <div key={sec.title || i}>
            {sec.title && <div className="navsection">{sec.title}</div>}
            <div className="siterow">
              {sec.items.map((s) => (
                <button key={`${i}-${s.id}`} type="button" role="option" aria-selected={isOn(s.id)}
                  className={`siteopt${isOn(s.id) ? ' sel' : ''}`} onClick={() => pick(s.id)}>
                  <SiteGlyph site={s} />
                  <span className="grow">{s.name}</span>
                  {isOn(s.id) && <Icon name="check" className="icon sm" />}
                </button>
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

type PickerProps = { sites: Site[]; recent: string[] }

/** Inline single-platform chooser: a button plus a panel below it (no floating popover). */
export function SitePicker({ value, onChange, sites, recent }: PickerProps & { value: string; onChange: (id: string) => void }) {
  const [open, setOpen] = useState(false)
  const current = siteById(sites, value)
  return (
    <div className="field">
      <button type="button" className="select sitebtn" aria-expanded={open} aria-label={t.create.site} onClick={() => setOpen(!open)}>
        <SiteGlyph site={current} />
        <span className="grow" style={{ textAlign: 'left' }}>{current.name}</span>
      </button>
      {open && <SitePanel sites={sites} recent={recent} isOn={(id) => id === value}
        onPick={(id) => { onChange(id); return true }} onClose={() => setOpen(false)} />}
    </div>
  )
}

/** The chosen platforms as chips, each removable. */
function SiteChips({ chosen, onRemove }: { chosen: Site[]; onRemove: (id: string) => void }) {
  return (
    <>
      {chosen.map((s) => (
        <span key={s.id} className="sitechip">
          <SiteGlyph site={s} />
          <span>{s.name}</span>
          <button type="button" className="chipx" aria-label={t.sites.remove(s.name)} title={t.sites.remove(s.name)} onClick={() => onRemove(s.id)}>
            <Icon name="x" />
          </button>
        </span>
      ))}
    </>
  )
}

/** A modal for choosing several platforms at once; nothing changes until Done. */
function SitesDialog({ value, sites, recent, onDone, onClose }: PickerProps & { value: string[]; onDone: (ids: string[]) => void; onClose: () => void }) {
  const [draft, setDraft] = useState(() => value.filter((id) => id !== 'other'))
  const toggle = (id: string) => {
    setDraft((d) => (d.includes(id) ? d.filter((x) => x !== id) : [...d, id]))
    return false
  }
  const done = () => onDone(draft)
  return (
    <Dialog title={t.sites.pick} icon="plus" className="sitedialog" onClose={onClose} onSubmit={done}
      footer={<>
        <span className="grow hint muted">{t.sites.chosen(draft.length)}</span>
        <button type="button" className="btn ghost" onClick={onClose}>{t.common.cancel}</button>
        <button type="submit" className="btn primary">{t.sites.done}</button>
      </>}>
      {draft.length > 0 && (
        <div className="sitechips" aria-label={t.sites.chosen(draft.length)}>
          <SiteChips chosen={draft.map((id) => siteById(sites, id))} onRemove={toggle} />
        </div>
      )}
      <SitePanel className="sitepanel big" sites={sites.filter((s) => s.id !== 'other')} recent={recent}
        isOn={(id) => draft.includes(id)} onPick={toggle} onClose={onClose} onEnterEmpty={done} />
      <span className="hint">{draft.length > 1 ? t.sites.samePerson : t.sites.multiHint}</span>
    </Dialog>
  )
}

/** A profile's platforms as chips, with a button that opens the chooser.
 *  Each platform at most once, so one profile never holds two accounts of
 *  the same platform. None means "Other". */
export function SitesPicker({ value, onChange, sites, recent }: PickerProps & { value: string[]; onChange: (ids: string[]) => void }) {
  const [open, setOpen] = useState(false)
  const btnRef = useRef<HTMLButtonElement>(null)
  const chosen = value.filter((id) => id !== 'other').map((id) => siteById(sites, id))
  const close = () => { setOpen(false); btnRef.current?.focus() }
  return (
    <div className="field">
      <div className="sitechips">
        {chosen.length === 0 && <span className="hint muted">{t.sites.noneChosen}</span>}
        <SiteChips chosen={chosen} onRemove={(id) => onChange(value.filter((x) => x !== id))} />
        <button ref={btnRef} type="button" className="btn sm" aria-haspopup="dialog" onClick={() => setOpen(true)}>
          <Icon name={chosen.length ? 'pencil' : 'plus'} className="icon sm" />{chosen.length ? t.sites.change : t.sites.add}
        </button>
      </div>
      {chosen.length > 1 && <span className="hint">{t.sites.samePerson}</span>}
      {open && <SitesDialog value={value} sites={sites} recent={recent} onClose={close}
        onDone={(ids) => { onChange(ids); close() }} />}
    </div>
  )
}
