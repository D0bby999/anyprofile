// SPDX-License-Identifier: AGPL-3.0-or-later
/** Returns current plus every id between from and to (inclusive) in visibleIds. */
export function rangeSelect(visibleIds: string[], from: string, to: string, current: Set<string>): Set<string> {
  const a = visibleIds.indexOf(from)
  const b = visibleIds.indexOf(to)
  if (a < 0 || b < 0) return current
  const lo = Math.min(a, b)
  const hi = Math.max(a, b)
  const next = new Set(current)
  for (let i = lo; i <= hi; i++) next.add(visibleIds[i])
  return next
}
