// SPDX-License-Identifier: AGPL-3.0-or-later
import { site } from '../api'

export type Site = site.Site

/** Text color that stays readable on a brand background. */
export function glyphColors(hex: string): { background: string; color: string } {
  const m = /^#?([0-9a-f]{6})$/i.exec(hex.trim())
  if (!m) return { background: hex, color: '#fff' }
  const n = parseInt(m[1], 16)
  const r = ((n >> 16) & 255) / 255
  const g = ((n >> 8) & 255) / 255
  const b = (n & 255) / 255
  const lum = 0.2126 * r + 0.7152 * g + 0.0722 * b
  return { background: hex, color: lum > 0.6 ? '#111' : '#fff' }
}

/** The catalog entry whose domains best match a URL, or null. Longest wins:
 *  business.facebook.com is facebook-ads, not facebook. */
export function guessSite(url: string, sites: Site[]): string | null {
  let host: string
  try {
    host = new URL(url).hostname.replace(/^www\./, '').toLowerCase()
  } catch {
    return null
  }
  let best: string | null = null
  let bestLen = -1
  for (const s of sites) {
    for (const d of s.domains ?? []) {
      if ((host === d || host.endsWith('.' + d)) && d.length > bestLen) {
        best = s.id
        bestLen = d.length
      }
    }
  }
  return best
}

/** The site with this id, falling back to other, then to a neutral badge. */
export function siteById(sites: Site[], id: string): Site {
  for (const s of sites) if (s.id === id) return s
  for (const s of sites) if (s.id === 'other') return s
  return site.Site.createFrom({
    id, name: id, group: 'other', domains: [], startUrl: '', icon: '', color: '#6B7280', custom: false,
  })
}
