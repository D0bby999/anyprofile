// SPDX-License-Identifier: AGPL-3.0-or-later
import { describe, expect, it } from 'vitest'
import { countryFlag } from './flags'

describe('countryFlag', () => {
  it('converts ISO-3166 alpha-2 codes to regional indicator flags', () => {
    expect(countryFlag('DE')).toBe('🇩🇪')
    expect(countryFlag('vn')).toBe('🇻🇳')
    expect(countryFlag(' US ')).toBe('🇺🇸')
  })

  it('returns empty string for invalid or empty codes', () => {
    expect(countryFlag('')).toBe('')
    expect(countryFlag('?')).toBe('')
    expect(countryFlag('USA')).toBe('')
    expect(countryFlag('12')).toBe('')
  })
})
