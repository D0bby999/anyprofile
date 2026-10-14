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
  id: 'a', name: 'Kênh A', site: 'tiktok', serial: 1, engine: 'fury', personaId: 'macos-m4pro-1728x1117',
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
  it('shows the identity, channel and proxy exit of a stopped profile, and opens it', () => {
    const p = props([row({})])
    render(<ProfileTable {...p} />)
    expect(screen.getByText('Mac M4 Pro · 1728×1117')).toBeTruthy()
    expect(screen.getByText('Fury')).toBeTruthy()
    expect(screen.getByText('67.84.146.69')).toBeTruthy()
    expect(screen.getByText('US')).toBeTruthy()
    expect(screen.getByText(t.health.new)).toBeTruthy()
    expect(screen.queryByText(t.profiles.colHealth)).toBeTruthy()
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
