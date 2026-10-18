// SPDX-License-Identifier: AGPL-3.0-or-later

/** Where a row sorts in the order the user dragged: by its place there, then rows never placed by serial. */
export function rank(r: { id: string; serial: number; position?: number }, order: Map<string, number> | null): number {
  const placed = order ? order.get(r.id) : r.position || undefined
  return placed !== undefined ? placed : 1e9 + r.serial
}

/** ids with id moved to index to, kept inside [lo, hi]. */
export function moveTo(ids: string[], id: string, to: number, lo = 0, hi = ids.length - 1): string[] {
  const out = ids.filter((x) => x !== id)
  out.splice(Math.min(Math.max(to, lo), hi), 0, id)
  return out
}

/**
 * The whole order after the visible rows were dragged into shown: the visible
 * rows take the places they held among the whole list, in their new order,
 * and rows a filter hides keep theirs.
 */
export function mergeOrder(whole: string[], shown: string[]): string[] {
  const visible = new Set(shown)
  let next = 0
  return whole.map((id) => (visible.has(id) ? shown[next++] : id))
}

/** First and last index of the rows sharing the pinned state of rows[i]: dragging stays among them. */
export function groupOf(rows: { pinned: boolean }[], i: number): [number, number] {
  let lo = i
  let hi = i
  while (lo > 0 && rows[lo - 1].pinned === rows[i].pinned) lo--
  while (hi < rows.length - 1 && rows[hi + 1].pinned === rows[i].pinned) hi++
  return [lo, hi]
}
