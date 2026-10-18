// SPDX-License-Identifier: AGPL-3.0-or-later
import { useLayoutEffect, useRef, useState, type PointerEvent as ReactPointerEvent, type RefObject } from 'react'
import { groupOf, moveTo } from '../lib/profile-order'

/** How far the pointer moves, mostly up or down, before a press on a row becomes a drag. */
const THRESHOLD = 6
const SETTLE = 'transform .18s cubic-bezier(.2, .8, .2, 1)'

type Row = { id: string; pinned: boolean }
type Live = {
  id: string; from: number; to: number; dy: number; ids: string[]
  tops: number[]; heights: number[]; trs: HTMLElement[]
  startY: number; startScroll: number; lo: number; hi: number
  pointerY: number; wrap: Element | null; frame: number
}

/**
 * Dragging rows of a table into a new order. The DOM keeps its order while
 * dragging: the dragged row follows the pointer and the rows it passes slide
 * aside, all by transforms, so nothing re-renders as the pointer moves.
 * Letting go reorders the rows and the dragged one glides into its place;
 * Escape slides everything back. Pinned rows stay among the pinned.
 */
export function useRowDrag(body: RefObject<HTMLTableSectionElement | null>, rows: Row[], onReorder?: (ids: string[]) => void, onBlocked?: () => void) {
  const [dragging, setDragging] = useState<string | null>(null)
  const live = useRef<Live | null>(null)
  // After a drop, the dragged row's offset from its new place, to glide from.
  const landing = useRef<{ id: string; top: number } | null>(null)

  const trOf = (id: string) => [...body.current?.querySelectorAll<HTMLElement>('tr[data-id]') ?? []].find((tr) => tr.dataset.id === id) ?? null

  useLayoutEffect(() => {
    const l = landing.current
    if (!l) return
    landing.current = null
    for (const tr of body.current?.querySelectorAll<HTMLElement>('tr[data-id]') ?? []) {
      tr.style.transition = 'none'
      tr.style.transform = ''
    }
    const tr = trOf(l.id)
    if (!tr) return
    const from = l.top - tr.offsetTop
    if (Math.abs(from) < 1) return
    tr.style.transform = `translateY(${from}px)`
    void tr.offsetHeight // the start of the glide is laid out first
    tr.style.transition = SETTLE
    tr.style.transform = ''
  })

  const layout = (l: Live) => {
    const h = l.heights[l.from]
    l.trs.forEach((tr, j) => {
      if (j === l.from) return
      const shift = l.from < l.to && j > l.from && j <= l.to ? -h : l.to < l.from && j >= l.to && j < l.from ? h : 0
      tr.style.transform = shift ? `translateY(${shift}px)` : ''
    })
  }

  const follow = (l: Live) => {
    const scrolled = (l.wrap?.scrollTop ?? 0) - l.startScroll
    // The row stays within its group: pinned among pinned, the rest below.
    const min = l.tops[l.lo] - l.tops[l.from]
    const max = l.tops[l.hi] + l.heights[l.hi] - l.tops[l.from] - l.heights[l.from]
    l.dy = Math.min(Math.max(l.pointerY - l.startY + scrolled, min), max)
    l.trs[l.from].style.transform = `translateY(${l.dy}px)`
    const mid = l.tops[l.from] + l.heights[l.from] / 2 + l.dy
    let to = l.from
    while (to > l.lo && mid <= l.tops[to - 1] + l.heights[to - 1] / 2) to--
    while (to < l.hi && mid >= l.tops[to + 1] + l.heights[to + 1] / 2) to++
    if (to !== l.to) {
      l.to = to
      layout(l)
    }
  }

  // Near the list's top or bottom edge it scrolls on, faster the closer.
  const scroll = () => {
    const l = live.current
    if (!l) return
    const box = l.wrap?.getBoundingClientRect()
    if (l.wrap && box) {
      const edge = 56
      const up = box.top + edge - l.pointerY
      const down = l.pointerY - (box.bottom - edge)
      const step = up > 0 ? -Math.ceil(up / 4) : down > 0 ? Math.ceil(down / 4) : 0
      if (step) {
        l.wrap.scrollTop += step
        follow(l)
      }
    }
    l.frame = requestAnimationFrame(scroll)
  }

  const finish = (keep: boolean) => {
    const l = live.current
    if (!l) return
    live.current = null
    cancelAnimationFrame(l.frame)
    setDragging(null)
    const moved = keep && l.to !== l.from
    if (moved) {
      landing.current = { id: l.id, top: l.tops[l.from] + l.dy }
      onReorder?.(moveTo(l.ids, l.id, l.to))
      return
    }
    for (const tr of l.trs) {
      tr.style.transition = SETTLE
      tr.style.transform = ''
    }
  }

  /** On a row's pointerdown: a drag starts once the pointer has moved. */
  const press = (e: ReactPointerEvent, id: string) => {
    if (e.button !== 0 || live.current || !body.current) return
    const x0 = e.clientX
    const y0 = e.clientY
    const fromGrip = !!(e.target as Element).closest('.grip')
    if (fromGrip) e.preventDefault()
    const begin = (y: number) => {
      const trs = [...body.current!.querySelectorAll<HTMLElement>('tr[data-id]')]
      const ids = trs.map((tr) => tr.dataset.id!)
      const from = ids.indexOf(id)
      if (from < 0) return
      const [lo, hi] = groupOf(ids.map((x) => rows.find((r) => r.id === x) ?? { pinned: false }), from)
      const wrap = body.current!.closest('.tablewrap')
      for (const tr of trs) tr.style.transition = ''
      trs[from].style.transition = 'none'
      window.getSelection()?.removeAllRanges()
      live.current = {
        id, from, to: from, dy: 0, ids, trs, lo, hi, wrap,
        tops: trs.map((tr) => tr.offsetTop), heights: trs.map((tr) => tr.offsetHeight),
        startY: y0, startScroll: wrap?.scrollTop ?? 0, pointerY: y, frame: 0,
      }
      setDragging(id)
      live.current.frame = requestAnimationFrame(scroll)
    }
    const move = (ev: PointerEvent) => {
      if (!live.current) {
        const dx = Math.abs(ev.clientX - x0)
        const dy = Math.abs(ev.clientY - y0)
        // Sideways is selecting text; up or down is moving the row.
        if (!fromGrip && (dy < THRESHOLD || dy < dx)) {
          if (dx >= THRESHOLD) stop()
          return
        }
        // Rows sorted another way than # cannot be dragged: say how to.
        if (!onReorder) {
          stop()
          onBlocked?.()
          return
        }
        begin(ev.clientY)
        if (!live.current) return stop()
      }
      live.current.pointerY = ev.clientY
      follow(live.current)
    }
    const up = () => {
      const was = !!live.current
      stop()
      finish(true)
      // The click that ends a drag is not a click on the row.
      if (was) {
        const swallow = (ev: MouseEvent) => { ev.stopPropagation(); ev.preventDefault() }
        window.addEventListener('click', swallow, { capture: true, once: true })
        setTimeout(() => window.removeEventListener('click', swallow, { capture: true }), 0)
      }
    }
    const key = (ev: KeyboardEvent) => {
      if (ev.key !== 'Escape' || !live.current) return
      ev.stopPropagation()
      stop()
      finish(false)
    }
    const cancel = () => { stop(); finish(false) }
    function stop() {
      window.removeEventListener('pointermove', move)
      window.removeEventListener('pointerup', up)
      window.removeEventListener('pointercancel', cancel)
      window.removeEventListener('keydown', key, true)
    }
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', up)
    window.addEventListener('pointercancel', cancel)
    window.addEventListener('keydown', key, true)
  }

  return { dragging, press }
}
