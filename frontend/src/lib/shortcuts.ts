// SPDX-License-Identifier: AGPL-3.0-or-later
import { isWindows, modDown } from './platform'

/** A key combo: "mod+k", "mod+enter", "/", "?", "up", "down", "enter", "space", "escape". */
export type Shortcut = {
  combo: string
  run: (e: KeyboardEvent) => void
  /** Also fires while typing in a field (⌘K, Escape). */
  global?: boolean
  when?: () => boolean
}

const tables = new Set<Shortcut[]>()
let listening = false

const names: Record<string, string> = { arrowup: 'up', arrowdown: 'down', ' ': 'space' }

/** The combo a keyboard event stands for, as written in a Shortcut. */
export function comboOf(e: Pick<KeyboardEvent, 'key' | 'metaKey' | 'ctrlKey' | 'altKey'>): string {
  const key = e.key.toLowerCase()
  return (modDown(e) ? 'mod+' : '') + (names[key] ?? key)
}

function typingIn(target: EventTarget | null): boolean {
  return target instanceof Element && !!target.closest('input, textarea, select, [contenteditable="true"], [contenteditable=""]')
}

export function handleKey(e: KeyboardEvent) {
  // Vietnamese and other input methods compose with keydown events; never act on them.
  if (e.isComposing || e.keyCode === 229) return
  // The other modifier (Ctrl on a Mac, the Windows key on Windows) belongs to the system.
  if (e.altKey || (isWindows ? e.metaKey : e.ctrlKey)) return
  const combo = comboOf(e)
  const typing = typingIn(e.target)
  // Enter and Space keep pressing the focused button, link or tab.
  const pressing = (combo === 'enter' || combo === 'space') && e.target instanceof Element &&
    !!e.target.closest('button, a, [role="tab"], [role^="menuitem"], [role="option"]')
  // A dialog on top owns the keyboard (it closes itself on Escape).
  const overlay = typeof document !== 'undefined' && document.querySelector('.backdrop')
  for (const table of [...tables].reverse()) {
    for (const s of table) {
      if (s.combo !== combo) continue
      if ((typing || pressing) && !s.global) continue
      if (overlay) continue
      if (s.when && !s.when()) continue
      e.preventDefault()
      s.run(e)
      return
    }
  }
}

/** Adds a table of shortcuts; the returned function removes it. One window listener serves all. */
export function registerShortcuts(table: Shortcut[]): () => void {
  if (!listening && typeof window !== 'undefined') {
    window.addEventListener('keydown', handleKey)
    listening = true
  }
  tables.add(table)
  return () => { tables.delete(table) }
}
