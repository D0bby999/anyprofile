// SPDX-License-Identifier: AGPL-3.0-or-later
import { useMemo, useRef, useState } from 'react'
import { t } from '../i18n'
import { siteById, type Site } from '../lib/sites'
import { SiteGlyph } from './site-glyph'

const groupOrder = ['video', 'social', 'messaging', 'commerce', 'ads', 'freelance', 'google', 'payments', 'custom', 'other']

type PanelProps = {
  sites: Site[]
  recent: string[]
  isOn: (id: string) => boolean
  /** Returns true when the panel should close. */
  onPick: (id: string) => boolean
  onClose: () => void
}

/** The searchable, grouped list of platforms below a picker button. */
function SitePanel({ sites, recent, isOn, onPick, onClose }: PanelProps) {
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
    if (e.key === 'Enter') {
      e.preventDefault()
      if (flat.length) pick(flat[0].id)
    } else if (e.key === 'Escape') {
      e.stopPropagation() // close the panel, not the dialog around it
      onClose()
    }
  }

  return (
    <div className="sitepanel" onKeyDown={onKeyDown}>
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
                  <span>{s.name}</span>
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

/** Inline chooser for a profile's platforms: each one at most once, so one
 *  profile never holds two accounts of the same platform. None means "Other". */
export function SitesPicker({ value, onChange, sites, recent }: PickerProps & { value: string[]; onChange: (ids: string[]) => void }) {
  const [open, setOpen] = useState(false)
  const btnRef = useRef<HTMLButtonElement>(null)
  const chosen = value.filter((id) => id !== 'other').map((id) => siteById(sites, id))
  const shown = chosen.length ? chosen : [siteById(sites, 'other')]
  const names = shown.map((s) => s.name).join(', ')
  const toggle = (id: string) => {
    onChange(value.includes(id) ? value.filter((x) => x !== id) : [...value.filter((x) => x !== 'other'), id])
    return false
  }
  return (
    <div className="field">
      <button ref={btnRef} type="button" className="select sitebtn" aria-expanded={open} aria-label={t.create.site} title={names}
        onClick={() => setOpen(!open)}>
        <span className="glyphs">{shown.map((s) => <SiteGlyph key={s.id} site={s} />)}</span>
        <span className="grow sitenames">{names}</span>
      </button>
      {open && (
        <>
          <SitePanel sites={sites.filter((s) => s.id !== 'other')} recent={recent} isOn={(id) => value.includes(id)}
            onPick={toggle} onClose={() => { setOpen(false); btnRef.current?.focus() }} />
          <span className="hint">{t.sites.multiHint}</span>
        </>
      )}
    </div>
  )
}
