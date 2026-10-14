// SPDX-License-Identifier: AGPL-3.0-or-later
import { useId, useState } from 'react'
import { t } from '../../i18n'
import { Icon } from '../icons'

type Props = { tags: string[]; known: string[]; onChange: (tags: string[]) => void }

/** Tags as chips. Enter adds; × or Backspace in the empty field removes. */
export function TagsGroup({ tags, known, onChange }: Props) {
  const [draft, setDraft] = useState('')
  const listId = useId()
  const add = () => {
    const tag = draft.trim()
    setDraft('')
    if (tag && !tags.includes(tag)) onChange([...tags, tag])
  }
  return (
    <section className="dgroup" aria-label={t.drawer.tags}>
      <h3>{t.drawer.tags}</h3>
      <div className="tagchips">
        {tags.map((tag) => (
          <span key={tag} className="chip tagchip">
            {tag}
            <button type="button" onClick={() => onChange(tags.filter((x) => x !== tag))} aria-label={t.drawer.removeTag(tag)}>
              <Icon name="x" className="icon xs" />
            </button>
          </span>
        ))}
        <input className="tagadd" value={draft} list={listId} placeholder={t.drawer.addTag} aria-label={t.drawer.addTag}
          onChange={(e) => setDraft(e.target.value)} onBlur={add}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ',') { e.preventDefault(); add() }
            if (e.key === 'Backspace' && !draft && tags.length) onChange(tags.slice(0, -1))
          }} />
        <datalist id={listId}>{known.filter((k) => !tags.includes(k)).map((k) => <option key={k} value={k} />)}</datalist>
      </div>
    </section>
  )
}
