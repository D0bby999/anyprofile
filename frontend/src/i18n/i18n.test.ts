// SPDX-License-Identifier: AGPL-3.0-or-later
import { afterEach, describe, expect, it } from 'vitest'
import { en } from './en'
import { vi as viCatalog } from './vi'
import { setLanguage, t } from './index'

function keysOf(o: unknown, prefix: string, out: string[]) {
  for (const [k, v] of Object.entries(o as Record<string, unknown>)) {
    const path = prefix ? `${prefix}.${k}` : k
    if (v !== null && typeof v === 'object') keysOf(v, path, out)
    else out.push(path)
  }
}

describe('i18n catalogs', () => {
  it('have identical key sets and value kinds', () => {
    const enKeys: string[] = []
    const viKeys: string[] = []
    keysOf(en, '', enKeys)
    keysOf(viCatalog, '', viKeys)
    expect(viKeys).toEqual(enKeys)
    for (const k of enKeys) {
      const a = k.split('.').reduce<any>((o, p) => o[p], en)
      const b = k.split('.').reduce<any>((o, p) => o[p], viCatalog)
      expect(typeof b, k).toBe(typeof a)
    }
  })

  it('default to English', () => {
    expect(t.nav.all).toBe('Web')
  })

  it('switch languages', () => {
    expect(setLanguage('vi')).toBe('vi')
    expect(t.nav.trash).toBe('Thùng rác')
    expect(setLanguage('xx')).toBe('en')
    expect(t.nav.trash).toBe('Trash')
  })
})

afterEach(() => setLanguage('en'))
