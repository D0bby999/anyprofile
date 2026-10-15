// SPDX-License-Identifier: AGPL-3.0-or-later
import { describe, expect, it } from 'vitest'
import { readTablePrefs } from './table-prefs'

const store = (v: unknown) => ({ getItem: () => JSON.stringify(v), setItem: () => {} })

describe('readTablePrefs', () => {
  it('hides Status only when both old Health and Last opened were hidden', () => {
    expect(readTablePrefs(store({ hidden: ['health', 'lastOpen', 'proxy'] })).hidden).toEqual(['proxy', 'status'])
    expect(readTablePrefs(store({ hidden: ['health'] })).hidden).toEqual([])
    expect(readTablePrefs(store({ density: 'compact', hidden: ['status'] }))).toEqual({ density: 'compact', hidden: ['status'] })
  })

  it('falls back to the defaults on broken data', () => {
    expect(readTablePrefs({ getItem: () => '{', setItem: () => {} })).toEqual({ density: 'comfortable', hidden: [] })
  })
})
