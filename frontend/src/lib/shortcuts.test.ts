// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { comboOf, registerShortcuts } from './shortcuts'

let off: () => void = () => {}
afterEach(() => { off(); document.body.innerHTML = '' })

const press = (target: EventTarget, init: KeyboardEventInit) =>
  target.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, ...init }))

describe('shortcuts', () => {
  it('names combos the way tables write them', () => {
    expect(comboOf({ key: 'K', metaKey: true, ctrlKey: false, altKey: false })).toBe('mod+k')
    expect(comboOf({ key: 'ArrowDown', metaKey: false, ctrlKey: false, altKey: false })).toBe('down')
    expect(comboOf({ key: ' ', metaKey: false, ctrlKey: false, altKey: false })).toBe('space')
  })

  it('runs ⌘N from the page but not while typing in a field', () => {
    const run = vi.fn()
    off = registerShortcuts([{ combo: 'mod+n', run }])
    press(document.body, { key: 'n', metaKey: true })
    expect(run).toHaveBeenCalledTimes(1)
    const input = document.body.appendChild(document.createElement('input'))
    press(input, { key: 'n', metaKey: true })
    expect(run).toHaveBeenCalledTimes(1)
  })

  it('lets ⌘K through a field, since it is global', () => {
    const run = vi.fn()
    off = registerShortcuts([{ combo: 'mod+k', global: true, run }])
    press(document.body.appendChild(document.createElement('input')), { key: 'k', metaKey: true })
    expect(run).toHaveBeenCalled()
  })

  it('ignores keys while an input method composes', () => {
    const run = vi.fn()
    off = registerShortcuts([{ combo: '/', run }])
    press(document.body, { key: '/', isComposing: true })
    expect(run).not.toHaveBeenCalled()
  })

  it('leaves Enter to a focused button', () => {
    const run = vi.fn()
    off = registerShortcuts([{ combo: 'enter', run }])
    press(document.body.appendChild(document.createElement('button')), { key: 'Enter' })
    expect(run).not.toHaveBeenCalled()
  })
})
