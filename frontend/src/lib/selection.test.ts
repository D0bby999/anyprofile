// SPDX-License-Identifier: AGPL-3.0-or-later
import { describe, expect, it } from 'vitest'
import { rangeSelect } from './selection'

describe('rangeSelect', () => {
  it('adds everything between two ids, inclusive', () => {
    const ids = ['a', 'b', 'c', 'd']
    expect([...rangeSelect(ids, 'a', 'c', new Set())].sort()).toEqual(['a', 'b', 'c'])
  })
  it('keeps the existing selection', () => {
    const ids = ['a', 'b', 'c']
    const got = rangeSelect(ids, 'a', 'b', new Set(['x']))
    expect(got.has('x')).toBe(true)
    expect(got.has('c')).toBe(false)
  })
  it('returns current when an id is not visible', () => {
    const cur = new Set(['x'])
    expect(rangeSelect(['a', 'b'], 'a', 'zz', cur)).toBe(cur)
  })
})
