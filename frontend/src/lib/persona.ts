// SPDX-License-Identifier: AGPL-3.0-or-later

/** "macos-m4pro-1728x1117" -> "Mac M4 Pro · 1728×1117". Unknown ids pass through. */
export function personaLabel(id: string): string {
  const m = /^macos-(m\d)(-?pro|-?max)?-(\d+)x(\d+)$/.exec(id)
  if (!m) return id
  const tier = m[2] ? ` ${m[2].replace('-', '').replace(/^./, (c) => c.toUpperCase())}` : ''
  return `Mac ${m[1].toUpperCase()}${tier} · ${m[3]}×${m[4]}`
}
