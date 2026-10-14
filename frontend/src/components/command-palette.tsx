// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import type { service } from '../api'
import { t } from '../i18n'
import { fold } from '../lib/format'
import { machineLabel } from '../lib/persona'
import { Icon, type IconName } from './icons'

export type Command = { id: string; label: string; icon: IconName; run: () => void }
type Item =
  | { kind: 'profile'; key: string; label: string; sub: string; row: service.ProfileRow }
  | { kind: 'command'; key: string; label: string; icon: IconName; run: () => void }

const MAX = 8

type Props = {
  profiles: service.ProfileRow[]
  actions: Command[]
  places: Command[]
  onOpenProfile: (id: string) => void
  onLaunchProfile: (row: service.ProfileRow) => void
  onClose: () => void
}

/** Rank by a folded query: a match at the start comes before one inside. -1 means no match. */
function rank(text: string, q: string): number {
  const i = fold(text).indexOf(q)
  return i < 0 ? -1 : i === 0 ? 0 : 1
}

function best<T>(list: T[], q: string, texts: (x: T) => string[]): T[] {
  if (!q) return list.slice(0, MAX)
  return list
    .map((x) => ({ x, r: Math.min(...texts(x).map((s) => rank(s, q)).map((r) => (r < 0 ? 9 : r))) }))
    .filter((m) => m.r < 9)
    .sort((a, b) => a.r - b.r)
    .slice(0, MAX)
    .map((m) => m.x)
}

/** ⌘K: find a profile, run an action or go somewhere, by keyboard. */
export function CommandPalette({ profiles, actions, places, onOpenProfile, onLaunchProfile, onClose }: Props) {
  const [query, setQuery] = useState('')
  const [cursor, setCursor] = useState(0)
  const listRef = useRef<HTMLDivElement>(null)
  const q = fold(query.trim())

  const groups = useMemo(() => {
    const serial = /^#(\d+)$/.exec(q)?.[1]
    const ps = serial ? profiles.filter((p) => String(p.serial) === serial).slice(0, MAX)
      : best(profiles, q, (p) => [p.name, p.login, ...p.tags])
    return [
      { title: t.palette.profiles, items: ps.map<Item>((row) => ({ kind: 'profile', key: `p:${row.id}`, label: row.name, sub: `#${row.serial} · ${machineLabel(row)}`, row })) },
      { title: t.palette.actions, items: best(actions, q, (a) => [a.label]).map<Item>((a) => ({ kind: 'command', key: `a:${a.id}`, label: a.label, icon: a.icon, run: a.run })) },
      { title: t.palette.goTo, items: best(places, q, (a) => [a.label]).map<Item>((a) => ({ kind: 'command', key: `g:${a.id}`, label: a.label, icon: a.icon, run: a.run })) },
    ].filter((g) => g.items.length > 0)
  }, [q, profiles, actions, places])
  const flat = groups.flatMap((g) => g.items)
  useEffect(() => setCursor(0), [q])
  useEffect(() => {
    listRef.current?.querySelector('[aria-selected="true"]')?.scrollIntoView?.({ block: 'nearest' })
  }, [cursor])

  const choose = (item: Item | undefined, launch: boolean) => {
    if (!item) return
    onClose()
    if (item.kind === 'command') item.run()
    else if (launch) onLaunchProfile(item.row)
    else onOpenProfile(item.row.id)
  }

  const onKey = (e: KeyboardEvent) => {
    if (e.nativeEvent.isComposing) return
    if (e.key === 'ArrowDown') { e.preventDefault(); setCursor((c) => Math.min(c + 1, flat.length - 1)) }
    if (e.key === 'ArrowUp') { e.preventDefault(); setCursor((c) => Math.max(c - 1, 0)) }
    if (e.key === 'Enter') { e.preventDefault(); choose(flat[cursor], e.metaKey) }
    if (e.key === 'Escape' || (e.key === 'k' && e.metaKey)) { e.preventDefault(); e.stopPropagation(); onClose() }
  }

  let index = -1
  return (
    <div className="backdrop palette-backdrop" onMouseDown={(e) => { if (e.target === e.currentTarget) onClose() }}>
      <div className="palette" role="dialog" aria-modal="true" aria-label={t.palette.title}>
        <div className="palette-input">
          <Icon name="search" />
          <input autoFocus value={query} placeholder={t.palette.placeholder} aria-label={t.palette.title}
            role="combobox" aria-expanded aria-controls="palette-list" aria-activedescendant={flat[cursor] ? `pal-${flat[cursor].key}` : undefined}
            onChange={(e) => setQuery(e.target.value)} onKeyDown={onKey} />
        </div>
        <div className="palette-list" id="palette-list" role="listbox" ref={listRef}>
          {flat.length === 0 && <p className="palette-empty">{t.palette.none(query.trim())}</p>}
          {groups.map((g) => (
            <div key={g.title} role="group" aria-label={g.title}>
              <div className="palette-group">{g.title}</div>
              {g.items.map((item) => {
                index++
                const i = index
                return (
                  <div key={item.key} id={`pal-${item.key}`} role="option" aria-selected={i === cursor} className="palette-item"
                    onMouseMove={() => setCursor(i)} onClick={(e) => choose(item, e.metaKey)}>
                    {item.kind === 'profile' ? <Icon name="profiles" className="icon sm" /> : <Icon name={item.icon} className="icon sm" />}
                    <span className="grow">{item.label}{item.kind === 'profile' && <small>{item.sub}</small>}</span>
                    {i === cursor && item.kind === 'profile' && <kbd>⌘↵</kbd>}
                  </div>
                )
              })}
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
