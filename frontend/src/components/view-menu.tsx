// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useRef, useState } from 'react'
import { t } from '../i18n'
import { COLUMNS, type Column, type TablePrefs } from '../lib/table-prefs'
import { Icon } from './icons'
import { focusFirstItem, menuArrows } from '../lib/menu-keys'

/** Table density and which optional columns show. */
export function ViewMenu({ prefs, onChange }: { prefs: TablePrefs; onChange: (p: TablePrefs) => void }) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => { if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false) }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [open])
  const toggle = (c: Column) => onChange({ ...prefs, hidden: prefs.hidden.includes(c) ? prefs.hidden.filter((x) => x !== c) : [...prefs.hidden, c] })

  return (
    <div className="rowmenu-wrap" ref={ref}>
      <button type="button" className="btn sm" aria-haspopup="menu" aria-expanded={open} onClick={() => setOpen(!open)}>
        <Icon name="settings" className="icon sm" />{t.view.button}
      </button>
      {open && (
        <div role="menu" className="rowmenu viewmenu" aria-label={t.view.button} ref={focusFirstItem}
          onKeyDown={(e) => { menuArrows(e); if (e.key === 'Escape') { e.stopPropagation(); setOpen(false) } }}>
          <span className="menulabel">{t.view.density}</span>
          {(['comfortable', 'compact'] as const).map((d) => (
            <button key={d} role="menuitemradio" aria-checked={prefs.density === d} onClick={() => onChange({ ...prefs, density: d })}>
              <Icon name="check" className={`icon sm${prefs.density === d ? '' : ' invisible'}`} />{t.view[d]}
            </button>
          ))}
          <hr />
          <span className="menulabel">{t.view.columns}</span>
          {COLUMNS.map((c) => (
            <button key={c} role="menuitemcheckbox" aria-checked={!prefs.hidden.includes(c)} onClick={() => toggle(c)}>
              <Icon name="check" className={`icon sm${prefs.hidden.includes(c) ? ' invisible' : ''}`} />{t.view.cols[c]}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
