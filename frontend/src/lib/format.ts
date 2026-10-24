// SPDX-License-Identifier: AGPL-3.0-or-later
import { language, t } from '../i18n'

/** A date in the app's language (23/11/2026 in Vietnamese), not the system's. */
export const formatDate = (iso: string) => new Date(iso).toLocaleDateString(language)

/** A date and time in the app's language. */
export const formatDateTime = (iso: string, opts?: Intl.DateTimeFormatOptions) => new Date(iso).toLocaleString(language, opts)

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

/** How long ago iso was, as "2 h 5 min"; for a running browser. */
export function sinceSpan(iso: string, now: Date = new Date()): string {
  const mins = Math.max(0, Math.floor((now.getTime() - new Date(iso).getTime()) / 60000)) || 0
  return t.time.span(Math.floor(mins / 60), mins % 60)
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

/** Lower case without diacritics, for search: "Thử" and "thu" match. */
export function fold(s: string): string {
  return s.normalize('NFD').replace(/\p{M}/gu, '').replace(/đ/g, 'd').replace(/Đ/g, 'D').toLowerCase()
}

/** The apperr code of an error ("engine_missing"), or "". */
export function errorCode(e: unknown): string {
  const raw = typeof e === 'string' ? e : e instanceof Error ? e.message : String(e)
  return /^anyprofile:([a-z_]+): /.exec(raw)?.[1] ?? ''
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

/** "base" alone, or "base 1" .. "base n". Mirrors the Go nameSeries. */
export function nameSeries(base: string, n: number): string[] {
  if (n === 1) return [base]
  return Array.from({ length: n }, (_, i) => `${base} ${i + 1}`)
}

/** A pasted proxy line that was not added, in the UI language when the problem is known. */
export const proxyLineError = (f: { line: number; err: string; code?: string }) =>
  t.proxies.failedLine(f.line, (f.code && t.proxies.parseErr[f.code]) || f.err)
