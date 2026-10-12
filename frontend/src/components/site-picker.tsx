// SPDX-License-Identifier: AGPL-3.0-or-later
import { useMemo, useRef, useState } from 'react'
import { t } from '../i18n'
import { siteById, type Site } from '../lib/sites'
import { SiteGlyph } from './site-glyph'

const groupOrder = ['video', 'social', 'messaging', 'commerce', 'ads', 'freelance', 'google', 'payments', 'custom', 'other']

type Props = {
  value: string
  onChange: (id: string) => void
  sites: Site[]
  recent: string[]
}

/** Inline platform chooser: a button plus a panel below it (no floating popover). */
export function SitePicker({ value, onChange, sites, recent }: Props) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)
  const current = siteById(sites, value)

  const q = query.trim().toLowerCase()
  const visible = useMemo(() => {
    if (q) return sites.filter((s) => s.name.toLowerCase().includes(q) || (s.domains ?? []).some((d) => d.includes(q)))
    return sites
  }, [sites, q])

  const sections = useMemo(() => {
    if (q) return [{ title: '', items: visible }]
    const out: { title: string; items: Site[] }[] = []
    const recentItems = recent.filter((id) => sites.some((s) => s.id === id)).slice(0, 6).map((id) => siteById(sites, id))
    if (recentItems.length) out.push({ title: t.siteGroups.recent, items: recentItems })
    for (const g of groupOrder) {
      const items = visible.filter((s) => s.group === g)
      if (items.length) out.push({ title: t.siteGroups[g] ?? g, items })
    }
    return out
  }, [q, visible, recent, sites])

  const flat = sections.flatMap((s) => s.items)

  const choose = (id: string) => {
    setOpen(false)
    setQuery('')
    onChange(id)
  }

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      if (flat.length) choose(flat[0].id)
    } else if (e.key === 'Escape') {
      e.stopPropagation() // close the panel, not the dialog around it
      setOpen(false)
      setQuery('')
    }
  }

  return (
    <div className="field">
      <button type="button" className="select sitebtn" aria-expanded={open} aria-label={t.create.site}
        onClick={() => { setOpen(!open); setTimeout(() => inputRef.current?.focus(), 0) }}>
        <SiteGlyph site={current} />
        <span className="grow" style={{ textAlign: 'left' }}>{current.name}</span>
      </button>
      {open && (
        <div className="sitepanel" onKeyDown={onKeyDown}>
          <input ref={inputRef} className="input" autoFocus placeholder={t.sites.search}
            value={query} onChange={(e) => setQuery(e.target.value)} aria-label={t.sites.search} />
          <div className="sitelist">
            {flat.length === 0 && <div className="hint muted" style={{ padding: '8px 4px' }}>{t.sites.none}</div>}
            {sections.map((sec, i) => (
              <div key={sec.title || i}>
                {sec.title && <div className="navsection">{sec.title}</div>}
                {sec.items.map((s) => (
                  <button key={`${i}-${s.id}`} type="button" role="option" aria-selected={s.id === value}
                    className={`siteopt${s.id === value ? ' sel' : ''}`} onClick={() => choose(s.id)}>
                    <SiteGlyph site={s} />
                    <span>{s.name}</span>
                  </button>
                ))}
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
