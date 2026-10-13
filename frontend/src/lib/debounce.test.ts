// SPDX-License-Identifier: AGPL-3.0-or-later
import { afterEach, describe, expect, it, vi } from 'vitest'
import { debounce } from './debounce'

afterEach(() => vi.useRealTimers())

describe('debounce', () => {
  it('calls fn once after a burst', () => {
    vi.useFakeTimers()
    const fn = vi.fn()
    const trigger = debounce(fn, 150)
    for (let i = 0; i < 5; i++) {
      trigger()
      vi.advanceTimersByTime(50)
    }
    vi.advanceTimersByTime(150)
    expect(fn).toHaveBeenCalledTimes(1)
  })

  it('cancel drops a pending call', () => {
    vi.useFakeTimers()
    const fn = vi.fn()
    const trigger = debounce(fn, 150)
    trigger()
    trigger.cancel()
    vi.advanceTimersByTime(300)
    expect(fn).not.toHaveBeenCalled()
  })
})
