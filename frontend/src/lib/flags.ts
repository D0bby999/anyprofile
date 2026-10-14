// SPDX-License-Identifier: AGPL-3.0-or-later
import { language, t } from '../i18n'

export type Flag = { id: string; level: string; params: Record<string, string> }

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
      : f.id === 'tz_mismatch' ? make(p.profile, p.ip)
        : f.id === 'exit_failed' ? make(p.error) : make()
  } else if (typeof make === 'string') {
    text = make
  }
  return { label, text }
}
