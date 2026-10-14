// SPDX-License-Identifier: AGPL-3.0-or-later
import type { service } from '../api'
import { t } from '../i18n'
import { flagText, type Flag } from './flags'

export type HealthLevel = 'danger' | 'warn' | 'info' | 'ok' | 'none'
export type Health = { level: HealthLevel; label: string; reasons: string[] }
type Reason = { level: HealthLevel; label: string; text: string }

const rank: Record<HealthLevel, number> = { danger: 4, warn: 3, info: 2, ok: 1, none: 0 }

type Row = Pick<service.ProfileRow, 'fpVerdict' | 'fpDiff' | 'proxyExpiry' | 'lastOpenAt' | 'cdpPort'> & { flags?: Flag[] }

/** The one thing most worth knowing about a profile, plus every reason in words. */
export function health(row: Row, now: Date = new Date()): Health {
  void now // later checks (sessions, consistency) depend on time
  const r: Reason[] = []
  const h = t.health
  if (row.fpVerdict === 'drift') {
    const fields = (row.fpDiff ?? []).map((f) => h.fields[f] ?? f).join(', ')
    r.push({ level: 'danger', label: h.drift, text: h.driftText(fields) })
  }
  for (const f of row.flags ?? []) {
    const { label, text } = flagText(f)
    r.push({ level: (f.level as HealthLevel) || 'info', label, text })
  }
  if (row.proxyExpiry === 'expired') r.push({ level: 'danger', label: h.expired, text: h.expiredText })
  if (row.proxyExpiry === 'soon') r.push({ level: 'warn', label: h.expiring, text: h.expiringText })
  if (row.cdpPort > 0) r.push({ level: 'warn', label: h.cdp(row.cdpPort), text: h.cdpText })
  if (row.fpVerdict === 'unchecked') r.push({ level: 'warn', label: h.unchecked, text: h.uncheckedText })
  if (row.fpVerdict === 'baseline') r.push({ level: 'info', label: h.baseline, text: h.baselineText })
  if (row.fpVerdict === 'match') r.push({ level: 'ok', label: h.ok, text: h.okText })
  if (!row.fpVerdict && !row.lastOpenAt) r.push({ level: 'none', label: h.new, text: h.newText })
  if (r.length === 0) return { level: 'none', label: '', reasons: [] }
  const top = r.reduce((a, b) => (rank[b.level] > rank[a.level] ? b : a))
  return { level: top.level, label: top.label, reasons: r.map((x) => x.text) }
}

/** Counted under Needs attention: a warning or a danger in the profile's health. */
export function needsAttention(row: Row): boolean {
  const level = health(row).level
  return level === 'warn' || level === 'danger'
}

/** Sidebar counts for the live profiles. */
export function countProfiles(rows: (Row & { state: string })[]) {
  const c = { all: 0, running: 0, attention: 0 }
  for (const r of rows) {
    c.all++
    if (r.state !== 'stopped') c.running++
    if (needsAttention(r)) c.attention++
  }
  return c
}
