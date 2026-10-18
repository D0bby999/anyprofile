// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { t } from '../i18n'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { service, site } from '../api'
import { ProfileTable } from './profile-table'

afterEach(cleanup)

const sites = [
  site.Site.createFrom({ id: 'tiktok', name: 'TikTok', group: 'video', domains: ['tiktok.com'], startUrl: '', icon: 'tiktok', color: '#000000', custom: false }),
  site.Site.createFrom({ id: 'other', name: 'Other', group: 'other', domains: [], startUrl: '', icon: '', color: '#6B7280', custom: false }),
]

const row = (over: Partial<service.ProfileRow>) => service.ProfileRow.createFrom({
  id: 'a', name: 'Kênh A', sites: ['tiktok'], serial: 1, engine: 'fury', personaId: 'macos-m4pro-1728x1117',
  proxyId: 'px', proxyLabel: 'u@gw:10001', proxyCountry: 'US', proxyExitIp: '67.84.146.69',
  state: 'stopped', stateError: '', cdpPort: 0, tags: ['tiktok'], lastOpenAt: '', createdAt: '2026-10-12T00:00:00Z', ...over,
})

const props = (rows: service.ProfileRow[]) => ({
  rows, sites, selected: new Set<string>(),
  sort: { key: 'no' as const, desc: false },
  onSort: vi.fn(), onToggle: vi.fn(), onToggleAll: vi.fn(), onStart: vi.fn(), onStop: vi.fn(), onFocus: vi.fn(), onEdit: vi.fn(),
  onDuplicate: vi.fn(), onTrash: vi.fn(), onMessage: vi.fn(), onError: vi.fn(),
})

describe('ProfileTable', () => {
  it('numbers the rows by their place in the list, not by the fixed serial', () => {
    const { container } = render(<ProfileTable {...props([row({ id: 'a', serial: 12 }), row({ id: 'b', name: 'Kênh B', serial: 40 })])} />)
    expect([...container.querySelectorAll('td.col-no')].map((td) => td.textContent)).toEqual(['1', '2'])
  })

  it('shows the identity, channel and proxy exit of a stopped profile, and opens it', () => {
    const p = props([row({})])
    render(<ProfileTable {...p} />)
    expect(screen.getByText('Mac M4 Pro · 1728×1117')).toBeTruthy()
    expect(screen.getByText('Fury')).toBeTruthy()
    expect(screen.getByText('67.84.146.69')).toBeTruthy()
    expect(screen.getByText('US')).toBeTruthy()
    expect(screen.getByText(t.health.new)).toBeTruthy()
    expect(screen.queryByText(t.profiles.colStatus)).toBeTruthy()
    fireEvent.click(screen.getByText(t.profiles.open))
    expect(p.onStart).toHaveBeenCalledWith('a')
    fireEvent.doubleClick(screen.getByText('Kênh A'))
    expect(p.onStart).toHaveBeenCalledTimes(2)
  })

  it('offers Close and Focus for a running profile, shows how long it runs and the open port', () => {
    const p = props([row({ state: 'running', cdpPort: 9333, lastOpenAt: new Date(Date.now() - 12 * 60000).toISOString() })])
    render(<ProfileTable {...p} />)
    fireEvent.click(screen.getByText(t.profiles.stop))
    expect(p.onStop).toHaveBeenCalledWith('a')
    fireEvent.click(screen.getByLabelText(t.profiles.focus))
    expect(p.onFocus).toHaveBeenCalledWith('a')
    expect(screen.getByText(t.health.cdp(9333))).toBeTruthy()
    expect(screen.getByText(t.profiles.runningFor(t.time.span(0, 12)))).toBeTruthy()
  })

  it('keeps the row controls one size whatever the state', () => {
    const { container } = render(<ProfileTable {...props([row({ id: 'a' }), row({ id: 'b', state: 'running' }), row({ id: 'c', state: 'starting' })])} />)
    const cells = [...container.querySelectorAll('td.col-actions')]
    // Same controls in every row: a closed one only hides its Focus slot.
    expect(cells.map((td) => td.querySelectorAll('button').length)).toEqual([3, 3, 3])
    const slot = cells[0].querySelector('button.slot') as HTMLButtonElement
    expect(slot.disabled && slot.tabIndex === -1 && slot.getAttribute('aria-hidden') === 'true').toBe(true)
    expect(cells[1].querySelector('button.slot')).toBeNull()
    // Every label is laid out so the button is as wide as the longest; only one is shown.
    for (const td of cells) expect(td.querySelectorAll('.actlabel > span').length).toBe(4)
    expect(screen.getAllByRole('button', { name: t.profiles.open })).toHaveLength(1)
    expect(screen.getByRole('button', { name: t.profiles.stop })).toBeTruthy()
    expect(screen.getByRole('button', { name: t.profiles.opening })).toBeTruthy()
  })

  it('drags a row into a new place, but not among the pinned ones', () => {
    const onReorder = vi.fn()
    const rows = [row({ id: 'p', name: 'P', pinned: true }), row({ id: 'a', name: 'A' }), row({ id: 'b', name: 'B' }), row({ id: 'c', name: 'C' })]
    const { container } = render(<ProfileTable {...props(rows)} onReorder={onReorder} />)
    // Rows 54 px tall from the top, in their current order on the screen.
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (this: HTMLElement) {
      const i = [...container.querySelectorAll('tr[data-id]')].indexOf(this)
      return { top: i * 54, height: 54, bottom: i * 54 + 54, left: 0, right: 0, width: 0, x: 0, y: i * 54, toJSON() {} } as DOMRect
    })
    const grip = screen.getByRole('button', { name: t.profiles.dragRow('C') })
    const names = () => [...container.querySelectorAll('td.cellname b')].map((b) => b.textContent)
    fireEvent.pointerDown(grip, { button: 0, pointerId: 1 })
    fireEvent.pointerMove(grip, { clientY: 0, pointerId: 1 }) // above P: stops below it
    expect(names()).toEqual(['P', 'C', 'A', 'B'])
    expect(container.querySelector('tr.dragging')?.getAttribute('data-id')).toBe('c')
    fireEvent.pointerUp(grip, { pointerId: 1 })
    expect(onReorder).toHaveBeenCalledWith(['p', 'c', 'a', 'b'])
    // Escape while dragging puts it back.
    onReorder.mockClear()
    fireEvent.pointerDown(grip, { button: 0, pointerId: 1 })
    fireEvent.pointerMove(grip, { clientY: 200, pointerId: 1 })
    fireEvent.keyDown(window, { key: 'Escape' })
    fireEvent.pointerUp(grip, { pointerId: 1 })
    expect(onReorder).not.toHaveBeenCalled()
    vi.restoreAllMocks()
  })

  it('marks the profiles an AI agent is driving', () => {
    render(<ProfileTable {...props([row({ id: 'a', state: 'running' }), row({ id: 'b', name: 'Kênh B' })])} agents={new Set(['a'])} />)
    // The row is tinted and titled; no chip joins the status cell.
    const rows = screen.getAllByTitle(t.agent.title)
    expect(rows).toHaveLength(1)
    expect(rows[0].tagName).toBe('TR')
    expect(rows[0].className).toContain('agent')
    expect(rows[0].textContent).toContain('Kênh A')
    expect(rows[0].querySelector('.col-status .chip.accent')).toBeNull()
  })

  it('disables the action while a profile is starting', () => {
    render(<ProfileTable {...props([row({ state: 'starting' })])} />)
    expect((screen.getByRole('button', { name: new RegExp(t.profiles.opening) }) as HTMLButtonElement).disabled).toBe(true)
  })

  it('labels a profile without proxy as direct and an unchecked proxy honestly', () => {
    render(<ProfileTable {...props([row({ id: 'b', proxyId: '' }), row({ id: 'c', proxyExitIp: '' })])} />)
    expect(screen.getByText(t.common.direct)).toBeTruthy()
    expect(screen.getByText(t.proxies.unchecked)).toBeTruthy()
  })

  it('sorts from the header', () => {
    const p = props([row({})])
    render(<ProfileTable {...p} />)
    fireEvent.click(screen.getByText(t.profiles.colName))
    expect(p.onSort).toHaveBeenCalledWith('name')
  })
})
