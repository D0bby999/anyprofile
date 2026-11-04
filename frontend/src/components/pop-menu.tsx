// SPDX-License-Identifier: AGPL-3.0-or-later
import { useCallback, useEffect, useLayoutEffect, useRef, useState, type KeyboardEvent, type ReactNode, type RefObject } from 'react'
import { createPortal } from 'react-dom'
import { focusFirstItem, menuArrows } from '../lib/menu-keys'

const GAP = 4 // between the button and the menu
const MARGIN = 8 // kept against the window edges
const FALLBACK_W = 200 // .rowmenu's min-width, before the menu has been measured

type Box = { top: number; bottom: number; right: number }
type Size = { w: number; h: number }
type View = { w: number; h: number }

/**
 * Where the menu goes: below its button, right edges aligned, kept inside the
 * window — and above the button when there is no room below it.
 */
export function menuPosition(trigger: Box, size: Size, view: View): { left: number; top: number } {
  const left = Math.max(MARGIN, Math.min(trigger.right - size.w, view.w - size.w - MARGIN))
  const below = trigger.bottom + GAP
  if (size.h === 0 || below + size.h <= view.h - MARGIN) return { left, top: below }
  const above = trigger.top - GAP - size.h
  return { left, top: above >= MARGIN ? above : Math.max(MARGIN, view.h - size.h - MARGIN) }
}

type Props = {
  /** The button the menu belongs to: the menu lines up with its right edge. */
  anchor: RefObject<HTMLElement | null>
  open: boolean
  onClose: () => void
  className?: string
  children: ReactNode
}

/**
 * A menu in a layer of its own, rendered into the body. Inside the table it
 * would be clipped by the scroll box, and the sticky action cells of the rows
 * below would paint over it; here nothing can cover it or cut it off.
 */
export function PopMenu({ anchor, open, onClose, className, children }: Props) {
  const menu = useRef<HTMLDivElement>(null)
  const focused = useRef(false)
  const [pos, setPos] = useState<{ left: number; top: number } | null>(null)

  const place = useCallback(() => {
    const t = anchor.current?.getBoundingClientRect()
    if (!t) return
    const m = menu.current?.getBoundingClientRect()
    setPos(menuPosition(t, { w: m?.width || FALLBACK_W, h: m?.height || 0 },
      { w: window.innerWidth, h: window.innerHeight }))
  }, [anchor])

  // Placed before the browser paints, so it never shows up at the wrong spot.
  useLayoutEffect(() => {
    if (!open) {
      focused.current = false
      setPos(null)
      return
    }
    place()
    window.addEventListener('resize', place)
    // The table scrolls under the menu: it follows its button rather than
    // drifting away from it.
    window.addEventListener('scroll', place, true)
    return () => {
      window.removeEventListener('resize', place)
      window.removeEventListener('scroll', place, true)
    }
  }, [open, place])

  // Focused once placed: it has no usable position before that.
  useLayoutEffect(() => {
    if (!open || !pos || focused.current) return
    focused.current = true
    focusFirstItem(menu.current)
  }, [open, pos])

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => {
      const t = e.target as Node | null
      if (!t || anchor.current?.contains(t) || menu.current?.contains(t)) return
      onClose()
    }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [open, onClose, anchor])

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    menuArrows(e)
    if (e.key === 'Escape') {
      e.stopPropagation()
      onClose()
    }
  }

  if (!open) return null
  return createPortal(
    <div role="menu" className={`rowmenu floating${className ? ` ${className}` : ''}`}
      style={{ left: pos?.left ?? -9999, top: pos?.top ?? -9999 }}
      ref={menu} onKeyDown={onKeyDown}>
      {children}
    </div>,
    document.body,
  )
}
