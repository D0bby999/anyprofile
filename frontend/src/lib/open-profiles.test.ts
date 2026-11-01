// SPDX-License-Identifier: AGPL-3.0-or-later
import { describe, expect, it } from 'vitest'
import { nthRunning } from './open-profiles'

const rows = [
  { id: 'a', state: 'stopped' },
  { id: 'b', state: 'running' },
  { id: 'c', state: 'starting' },
  { id: 'd', state: 'running' },
  { id: 'e', state: 'running' },
]

describe('nthRunning', () => {
  it('counts only profiles with a window, in list order', () => {
    expect(nthRunning(rows, 1)?.id).toBe('b')
    expect(nthRunning(rows, 2)?.id).toBe('d')
    expect(nthRunning(rows, 3)?.id).toBe('e')
  })

  it('has nothing to give past the open profiles', () => {
    expect(nthRunning(rows, 4)).toBeUndefined()
    expect(nthRunning(rows, 0)).toBeUndefined()
    expect(nthRunning([], 1)).toBeUndefined()
  })
})
