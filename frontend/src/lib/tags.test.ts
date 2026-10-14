// SPDX-License-Identifier: AGPL-3.0-or-later
import { describe, expect, it } from 'vitest'
import { tagColor, tagCounts } from './tags'

describe('tagColor', () => {
  it('is stable for a name', () => {
    expect(tagColor('shop-us')).toBe(tagColor('shop-us'))
    expect(tagColor('Kênh phụ')).toBe(tagColor('Kênh phụ'))
  })
  it('uses all eight colours over 100 names', () => {
    const seen = new Set(Array.from({ length: 100 }, (_, i) => tagColor(`tag-${i}`)))
    expect([...seen].sort()).toEqual([1, 2, 3, 4, 5, 6, 7, 8])
  })
})

describe('tagCounts', () => {
  it('orders by use, then by name', () => {
    expect(tagCounts([{ tags: ['b', 'a'] }, { tags: ['b'] }, { tags: [] }])).toEqual([['b', 2], ['a', 1]])
  })
})
