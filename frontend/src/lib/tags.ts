// SPDX-License-Identifier: AGPL-3.0-or-later

/** A stable colour slot (1 to 8) for a tag: FNV-1a 32-bit over the UTF-8 name. */
export function tagColor(name: string): number {
  let h = 0x811c9dc5
  for (const b of new TextEncoder().encode(name)) {
    h ^= b
    h = Math.imul(h, 0x01000193) >>> 0
  }
  return (h % 8) + 1
}

/** Tags with their profile counts, most used first, then by name. */
export function tagCounts(rows: { tags: string[] }[]): [string, number][] {
  const m = new Map<string, number>()
  for (const r of rows) for (const tag of r.tags ?? []) m.set(tag, (m.get(tag) ?? 0) + 1)
  return [...m.entries()].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
}
