// SPDX-License-Identifier: AGPL-3.0-or-later
import type { KeyboardEvent } from 'react'

/** Moves focus between the items of an open menu with the arrow keys. */
export function menuArrows(e: KeyboardEvent<HTMLElement>) {
  if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return
  const items = [...e.currentTarget.querySelectorAll<HTMLButtonElement>('[role^="menuitem"]:not(:disabled)')]
  const i = items.indexOf(document.activeElement as HTMLButtonElement)
  const next = items[(i + (e.key === 'ArrowDown' ? 1 : items.length - 1)) % items.length]
  e.preventDefault()
  next?.focus()
}

/** Focuses the first item when a menu opens, so Escape and the arrows work at once. */
export function focusFirstItem(menu: HTMLElement | null) {
  menu?.querySelector<HTMLElement>('[role^="menuitem"]:not(:disabled)')?.focus()
}
