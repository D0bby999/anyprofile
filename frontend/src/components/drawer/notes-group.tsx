// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useMemo, useRef, useState } from 'react'
import { t } from '../../i18n'
import { debounce } from '../../lib/debounce'

/** Notes save themselves 600 ms after typing stops, and on leaving the profile. */
export function NotesGroup({ value, onSave }: { value: string; onSave: (notes: string) => Promise<boolean> }) {
  const [draft, setDraft] = useState(value)
  const [saved, setSaved] = useState(false)
  const latest = useRef(draft)
  const pending = useRef(false)
  const save = useMemo(() => debounce(async () => {
    pending.current = false
    if (await onSave(latest.current)) {
      setSaved(true)
      setTimeout(() => setSaved(false), 1500)
    }
  }, 600), [])
  // Flush a pending save when the drawer switches profile or closes.
  useEffect(() => () => { if (pending.current) { save.cancel(); onSave(latest.current) } }, [])

  return (
    <section className="dgroup" aria-label={t.drawer.notes}>
      <h3>{t.drawer.notes}{saved && <span className="savedmark" role="status">{t.drawer.saved}</span>}</h3>
      <textarea className="textarea" rows={3} value={draft} aria-label={t.drawer.notes}
        onChange={(e) => { setDraft(e.target.value); latest.current = e.target.value; pending.current = true; save() }} />
    </section>
  )
}
