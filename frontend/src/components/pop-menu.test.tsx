// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { useRef, useState } from 'react'
import { PopMenu, menuPosition } from './pop-menu'

const view = { w: 1000, h: 800 }

describe('menuPosition', () => {
  it('sits below the button with the right edges lined up', () => {
    expect(menuPosition({ top: 100, bottom: 130, right: 900 }, { w: 200, h: 150 }, view))
      .toEqual({ left: 700, top: 134 })
  })

  it('opens upwards when it would not fit below', () => {
    expect(menuPosition({ top: 700, bottom: 730, right: 900 }, { w: 200, h: 150 }, view))
      .toEqual({ left: 700, top: 546 })
  })

  it('stops at the window edge instead of running off it', () => {
    expect(menuPosition({ top: 100, bottom: 130, right: 1000 }, { w: 200, h: 100 }, view).left).toBe(792)
    expect(menuPosition({ top: 100, bottom: 130, right: 40 }, { w: 200, h: 100 }, view).left).toBe(8)
  })

  it('keeps the top visible when the menu is taller than the window', () => {
    expect(menuPosition({ top: 20, bottom: 60, right: 500 }, { w: 200, h: 900 }, view).top).toBe(8)
  })
})

function Harness({ onClose }: { onClose: () => void }) {
  const [open, setOpen] = useState(true)
  const anchor = useRef<HTMLButtonElement>(null)
  return (
    <>
      <button ref={anchor}>More</button>
      <PopMenu anchor={anchor} open={open} onClose={() => { setOpen(false); onClose() }}>
        <button role="menuitem">Edit</button>
        <button role="menuitem">Trash</button>
      </PopMenu>
    </>
  )
}

afterEach(cleanup)

describe('PopMenu', () => {
  it('renders into the body and focuses the first item', () => {
    render(<Harness onClose={vi.fn()} />)
    const menu = screen.getByRole('menu')
    expect(menu.parentElement).toBe(document.body)
    expect(document.activeElement).toBe(screen.getByText('Edit'))
  })

  it('closes on Escape and on a click outside, not on one inside', () => {
    const onClose = vi.fn()
    render(<Harness onClose={onClose} />)
    fireEvent.mouseDown(screen.getByRole('menu'))
    expect(screen.getByRole('menu')).toBeTruthy()
    fireEvent.mouseDown(document.body)
    expect(onClose).toHaveBeenCalledTimes(1)
    cleanup()

    const again = vi.fn()
    render(<Harness onClose={again} />)
    fireEvent.keyDown(screen.getByText('Edit'), { key: 'Escape' })
    expect(again).toHaveBeenCalledTimes(1)
  })
})
