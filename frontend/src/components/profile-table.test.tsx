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

  it('leaves out a machine that is not known yet', () => {
    render(<ProfileTable {...props([row({ engine: 'apostate', personaId: '', machine: '' })])} />)
    expect(screen.queryByText(t.engines.machinePending)).toBeNull()
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
    // Open and Close are both laid out so the button is as wide as the longer; only one is shown.
    for (const td of cells) expect(td.querySelectorAll('.actlabel > span').length).toBe(2)
    expect(screen.getAllByRole('button', { name: t.profiles.open })).toHaveLength(1)
    expect(screen.getByRole('button', { name: t.profiles.stop })).toBeTruthy()
    expect(screen.getByRole('button', { name: t.profiles.opening })).toBeTruthy()
  })

  it('drags a row into a new place, but not among the pinned ones', async () => {
    const onReorder = vi.fn()
    const rows = [row({ id: 'p', name: 'P', pinned: true }), row({ id: 'a', name: 'A' }), row({ id: 'b', name: 'B' }), row({ id: 'c', name: 'C' })]
    const { container } = render(<ProfileTable {...props(rows)} onReorder={onReorder} />)
    // Rows 54 px tall, in their order in the DOM.
    const place = (el: HTMLElement) => [...container.querySelectorAll('tr[data-id]')].indexOf(el)
    vi.spyOn(HTMLElement.prototype, 'offsetTop', 'get').mockImplementation(function (this: HTMLElement) { return place(this) * 54 })
    vi.spyOn(HTMLElement.prototype, 'offsetHeight', 'get').mockImplementation(() => 54)
    const tr = (id: string) => container.querySelector<HTMLElement>(`tr[data-id="${id}"]`)!
    const name = container.querySelectorAll('td.cellname b')[3] as HTMLElement // C's name: not a control
    // A press that moves sideways is selecting text, not dragging.
    fireEvent.pointerDown(name, { button: 0, clientX: 10, clientY: 180 })
    fireEvent.pointerMove(window, { clientX: 60, clientY: 182 })
    expect(container.querySelector('tr.dragging')).toBeNull()
    fireEvent.pointerUp(window)
    // Up from C's name, past A and B and beyond P: it stops below P.
    fireEvent.pointerDown(name, { button: 0, clientX: 10, clientY: 180 })
    fireEvent.pointerMove(window, { clientX: 10, clientY: 170 })
    expect(tr('c').className).toContain('dragging')
    fireEvent.pointerMove(window, { clientX: 10, clientY: 0 })
    expect(tr('c').style.transform).toBe('translateY(-108px)') // held at the top of its group
    expect([tr('a').style.transform, tr('b').style.transform, tr('p').style.transform]).toEqual(['translateY(54px)', 'translateY(54px)', ''])
    fireEvent.pointerUp(window)
    expect(onReorder).toHaveBeenCalledWith(['p', 'c', 'a', 'b'])
    // Escape while dragging slides it back and keeps the order.
    onReorder.mockClear()
    fireEvent.pointerDown(screen.getByRole('button', { name: t.profiles.dragRow('A') }), { button: 0, clientX: 10, clientY: 70 })
    fireEvent.pointerMove(window, { clientX: 10, clientY: 200 })
    expect(tr('a').className).toContain('dragging')
    fireEvent.keyDown(window, { key: 'Escape' })
    fireEvent.pointerUp(window)
    expect(onReorder).not.toHaveBeenCalled()
    expect(tr('a').style.transform).toBe('')
    vi.restoreAllMocks()
    await new Promise((r) => setTimeout(r)) // the drop's click guard lifts
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
