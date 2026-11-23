// SPDX-License-Identifier: AGPL-3.0-or-later
import { language, t } from '../i18n'

export type Flag = { id: string; level: string; params: Record<string, string> }

/** "DE" -> "🇩🇪"; unknown or empty codes return "". */
export function countryFlag(cc: string): string {
  const s = (cc ?? '').trim().toUpperCase()
  if (!/^[A-Z]{2}$/.test(s)) return ''
  return String.fromCodePoint(0x1F1E6 + s.charCodeAt(0) - 65, 0x1F1E6 + s.charCodeAt(1) - 65)
}

/** "DE" -> "Germany" in the UI language; unknown codes pass through. */
export function countryName(cc: string): string {
  if (!cc) return ''
  try {
    return new Intl.DisplayNames([language], { type: 'region' }).of(cc) ?? cc
  } catch {
    return cc
  }
}

/** The short label and the sentence for an exit flag. */
export function flagText(f: Flag): { label: string; text: string } {
  const label = (t.flags[f.id] as string) ?? f.id
  const make = t.flags[`${f.id}Text`] as ((...a: string[]) => string) | string | undefined
  const p = f.params ?? {}
  let text = ''
  if (typeof make === 'function') {
    text = f.id === 'country_changed' ? make(countryName(p.to), countryName(p.from))
      : f.id === 'tz_mismatch' || f.id === 'ipv6_elsewhere' ? make(p.profile, p.ip)
        : f.id === 'exit_failed' ? make(p.error) : make()
  } else if (typeof make === 'string') {
    text = make
  }
  return { label, text }
}
