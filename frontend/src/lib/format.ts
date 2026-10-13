// SPDX-License-Identifier: AGPL-3.0-or-later
import { t } from '../i18n'

export function relativeTime(iso: string, now: Date = new Date()): string {
  if (!iso) return t.time.never
  const then = new Date(iso)
  if (Number.isNaN(then.getTime())) return t.time.never
  const mins = Math.floor((now.getTime() - then.getTime()) / 60000)
  if (mins < 1) return t.time.justNow
  if (mins < 60) return t.time.minutes(mins)
  if (mins < 60 * 24) return t.time.hours(Math.floor(mins / 60))
  return t.time.days(Math.floor(mins / (60 * 24)))
}

export function parseTags(text: string): string[] {
  const seen = new Set<string>()
  for (const raw of text.split(',')) {
    const tag = raw.trim()
    if (tag) seen.add(tag)
  }
  return [...seen]
}

export function allTags(rows: { tags: string[] }[]): string[] {
  return [...new Set(rows.flatMap((r) => r.tags))].sort((a, b) => a.localeCompare(b))
}

export function errorText(e: unknown): string {
  const raw = typeof e === 'string' ? e : e instanceof Error ? e.message : String(e)
  const m = /^anyprofile:([a-z_]+): ([\s\S]*)$/.exec(raw)
  if (!m) return raw
  return t.errors[m[1]] ?? m[2]
}

/** Human-readable size, base 1024, one decimal from MB up. */
export function formatBytes(n: number): string {
  if (!Number.isFinite(n) || n < 0) n = 0
  if (n < 1024) return `${Math.round(n)} B`
  if (n < 1024 * 1024) return `${Math.round(n / 1024)} KB`
  if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`
  return `${(n / (1024 * 1024 * 1024)).toFixed(1)} GB`
}
