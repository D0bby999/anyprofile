// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { service } from '../api'
import { ProfileTable } from './profile-table'

afterEach(cleanup)

const row = (over: Partial<service.ProfileRow>) => service.ProfileRow.createFrom({
  id: 'a', name: 'Kênh A', channelKind: 'tiktok', personaId: 'macos-m4pro-1728x1117',
  proxyId: 'px', proxyLabel: 'u@gw:10001', proxyCountry: 'US', proxyExitIp: '67.84.146.69',
  state: 'stopped', stateError: '', cdpPort: 0, tags: ['tiktok'], lastOpenAt: '', createdAt: '2026-10-12T00:00:00Z', ...over,
})

const props = (rows: service.ProfileRow[]) => ({
  rows, numbers: new Map(rows.map((r, i) => [r.id, i + 1])), selected: new Set<string>(),
  sort: { key: 'no' as const, desc: false },
  onSort: vi.fn(), onToggle: vi.fn(), onToggleAll: vi.fn(), onStart: vi.fn(), onStop: vi.fn(), onFocus: vi.fn(), onEdit: vi.fn(),
})

describe('ProfileTable', () => {
  it('shows the identity, channel and proxy exit of a stopped profile, and opens it', () => {
    const p = props([row({})])
    render(<ProfileTable {...p} />)
    expect(screen.getByText('Mac M4 Pro · 1728×1117')).toBeTruthy()
    expect(screen.getByText('67.84.146.69')).toBeTruthy()
    expect(screen.getByText('US')).toBeTruthy()
    expect(screen.getByText('Đã đóng')).toBeTruthy()
    fireEvent.click(screen.getByText('Mở'))
    expect(p.onStart).toHaveBeenCalledWith('a')
    fireEvent.doubleClick(screen.getByText('Kênh A'))
    expect(p.onStart).toHaveBeenCalledTimes(2)
  })

  it('offers Close and Focus for a running profile and shows the CDP warning chip', () => {
    const p = props([row({ state: 'running', cdpPort: 9333 })])
    render(<ProfileTable {...p} />)
    fireEvent.click(screen.getByText('Đóng'))
    expect(p.onStop).toHaveBeenCalledWith('a')
    fireEvent.click(screen.getByLabelText('Đưa cửa sổ lên trước'))
    expect(p.onFocus).toHaveBeenCalledWith('a')
    expect(screen.getByText('CDP :9333')).toBeTruthy()
    expect(screen.getByText('Đang chạy')).toBeTruthy()
  })

  it('disables the action while a profile is starting', () => {
    render(<ProfileTable {...props([row({ state: 'starting' })])} />)
    expect((screen.getByRole('button', { name: /Đang mở/ }) as HTMLButtonElement).disabled).toBe(true)
  })

  it('labels a profile without proxy as direct and an unchecked proxy honestly', () => {
    render(<ProfileTable {...props([row({ id: 'b', proxyId: '' }), row({ id: 'c', proxyExitIp: '' })])} />)
    expect(screen.getByText('Không proxy')).toBeTruthy()
    expect(screen.getByText('Chưa kiểm tra')).toBeTruthy()
  })

  it('sorts from the header', () => {
    const p = props([row({})])
    render(<ProfileTable {...p} />)
    fireEvent.click(screen.getByText('Tên'))
    expect(p.onSort).toHaveBeenCalledWith('name')
  })
})
