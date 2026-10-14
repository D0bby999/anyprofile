// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useRef, useState } from 'react'
import { Api, onDataChanged, store } from '../../api'
import { t } from '../../i18n'
import { errorText, formatBytes, relativeTime } from '../../lib/format'
import { focusFirstItem, menuArrows } from '../../lib/menu-keys'
import { useConfirm } from '../confirm'
import { Icon } from '../icons'
import { useToast } from '../toast'

const when = (iso: string) => new Date(iso).toLocaleString([], { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })

function SnapshotMenu({ snap, disabled, onRestore, onDelete }: { snap: store.Snapshot; disabled: boolean; onRestore: () => void; onDelete: () => void }) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => { if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false) }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [open])
  return (
    <div className="rowmenu-wrap" ref={ref}>
      <button type="button" className="iconbtn sm" aria-haspopup="menu" aria-expanded={open} aria-label={t.snapshots.more(when(snap.createdAt))}
        onClick={() => setOpen(!open)}><Icon name="more" /></button>
      {open && (
        <div role="menu" className="rowmenu" ref={focusFirstItem} onKeyDown={(e) => { menuArrows(e); if (e.key === 'Escape') { e.stopPropagation(); setOpen(false) } }}>
          <button role="menuitem" disabled={disabled} title={disabled ? t.snapshots.closeFirst : undefined} onClick={() => { setOpen(false); onRestore() }}>{t.snapshots.restore}</button>
          <button role="menuitem" onClick={() => { setOpen(false); onDelete() }}>{t.snapshots.delete}</button>
        </div>
      )}
    </div>
  )
}

/** Encrypted copies of the profile; restore one, with undo. */
export function SnapshotsTab({ profileId, running }: { profileId: string; running: boolean }) {
  const [list, setList] = useState<store.Snapshot[] | null>(null)
  const [busy, setBusy] = useState(false)
  const toast = useToast()
  const confirm = useConfirm()
  useEffect(() => {
    const load = () => Api.Snapshots(profileId).then((s) => setList(s ?? [])).catch(() => setList([]))
    load()
    return onDataChanged(['snapshots'], load)
  }, [profileId])

  const take = async () => {
    setBusy(true)
    try {
      await Api.TakeSnapshot(profileId)
      toast.success(t.snapshots.saved)
    } catch (e) {
      toast.error(errorText(e))
    } finally {
      setBusy(false)
    }
  }

  const restore = async (snap: store.Snapshot) => {
    if (!(await confirm(t.confirm.restore(when(snap.createdAt))))) return
    try {
      const before = await Api.RestoreSnapshot(snap.id)
      // Undo restores the copy of the current session saved just before.
      const undo = before ? { label: t.snapshots.undo, run: () => { Api.RestoreSnapshot(before).catch((e) => toast.error(errorText(e))) } } : undefined
      toast.success(t.snapshots.restored, undo)
    } catch (e) {
      toast.error(errorText(e))
    }
  }

  if (list === null) return <p className="hint">{t.common.loading}</p>
  return (
    <div className="snaptab">
      <button type="button" className={`btn sm${busy ? ' busy' : ''}`} aria-busy={busy} disabled={running || busy}
        title={running ? t.snapshots.closeFirst : undefined} onClick={take}>
        <Icon name="download" className="icon sm" />{t.snapshots.take}
      </button>
      {list.length === 0 ? <p className="sessions-empty">{t.snapshots.empty}</p> : (
        <ul className="snaplist">
          {list.map((s) => (
            <li key={s.id}>
              <div className="grow">
                <span title={when(s.createdAt)}>{relativeTime(s.createdAt)}</span>
                <small>{formatBytes(Number(s.bytes))}</small>
              </div>
              <span className="chip">{t.snapshots.reasons[s.reason] ?? s.reason}</span>
              <SnapshotMenu snap={s} disabled={running} onRestore={() => restore(s)}
                onDelete={() => Api.DeleteSnapshot(s.id).catch((e) => toast.error(errorText(e)))} />
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
