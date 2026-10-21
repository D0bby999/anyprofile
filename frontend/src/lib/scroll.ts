// SPDX-License-Identifier: AGPL-3.0-or-later

/** Brings a block to the top of its page's own scroll (.pagebody). scrollIntoView
 *  would also scroll the app shell, whose overflow is hidden, and leave it shifted. */
export function scrollSettingsTo(id: string, block: 'start' | 'center' = 'start') {
  const el = document.getElementById(id)
  const root = el?.closest<HTMLElement>('.pagebody')
  if (!el || !root) return
  const r = el.getBoundingClientRect()
  const box = root.getBoundingClientRect()
  root.scrollTop += block === 'center' ? r.top - box.top - (box.height - r.height) / 2 : r.top - box.top - 10
}
