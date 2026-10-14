// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useRef, useState } from 'react'
import { t } from '../../i18n'

/** The profile name; click to edit, Enter saves, Escape cancels, empty keeps the old name. */
export function InlineName({ value, onSave }: { value: string; onSave: (name: string) => void }) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(value)
  const cancelled = useRef(false)
  useEffect(() => { if (!editing) setDraft(value) }, [value, editing])

  if (!editing) {
    return (
      <button type="button" className="drawer-name" title={t.drawer.rename}
        onClick={() => { cancelled.current = false; setEditing(true) }}>{value}</button>
    )
  }
  const commit = () => {
    if (cancelled.current) return
    setEditing(false)
    const name = draft.trim()
    if (name && name !== value) onSave(name)
    else setDraft(value)
  }
  return (
    <input className="input drawer-name-input" autoFocus value={draft} aria-label={t.drawer.rename}
      onChange={(e) => setDraft(e.target.value)} onBlur={commit}
      onKeyDown={(e) => {
        if (e.key === 'Enter') { e.preventDefault(); commit() }
        if (e.key === 'Escape') { e.stopPropagation(); cancelled.current = true; setDraft(value); setEditing(false) }
      }} />
  )
}
