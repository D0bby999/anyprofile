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
  if (typeof e === 'string') return e
  if (e instanceof Error) return e.message
  return String(e)
}
