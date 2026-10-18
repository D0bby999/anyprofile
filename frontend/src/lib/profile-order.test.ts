// SPDX-License-Identifier: AGPL-3.0-or-later
import { describe, expect, it } from 'vitest'
import { groupOf, mergeOrder, moveTo, rank } from './profile-order'

describe('profile order', () => {
  it('ranks placed rows first, then the rest by serial', () => {
    const rows = [{ id: 'a', serial: 1 }, { id: 'b', serial: 2, position: 2 }, { id: 'c', serial: 3, position: 1 }, { id: 'd', serial: 4 }]
    expect([...rows].sort((x, y) => rank(x, null) - rank(y, null)).map((r) => r.id)).toEqual(['c', 'b', 'a', 'd'])
    // An order just dragged wins over the positions the list was loaded with.
    const order = new Map([['a', 0], ['d', 1]])
    expect([...rows].sort((x, y) => rank(x, order) - rank(y, order)).map((r) => r.id)).toEqual(['a', 'd', 'b', 'c'])
  })

  it('moves a row, but not out of its range', () => {
    expect(moveTo(['a', 'b', 'c', 'd'], 'd', 0)).toEqual(['d', 'a', 'b', 'c'])
    expect(moveTo(['a', 'b', 'c', 'd'], 'a', 9)).toEqual(['b', 'c', 'd', 'a'])
    expect(moveTo(['a', 'b', 'c', 'd'], 'd', 0, 2, 3)).toEqual(['a', 'b', 'd', 'c'])
  })

  it('keeps the places of rows a filter hides', () => {
    expect(mergeOrder(['a', 'x', 'b', 'y', 'c'], ['c', 'a', 'b'])).toEqual(['c', 'x', 'a', 'y', 'b'])
  })

  it('finds the pinned group of a row', () => {
    const rows = [{ pinned: true }, { pinned: true }, { pinned: false }, { pinned: false }, { pinned: false }]
    expect(groupOf(rows, 1)).toEqual([0, 1])
    expect(groupOf(rows, 3)).toEqual([2, 4])
  })
})
