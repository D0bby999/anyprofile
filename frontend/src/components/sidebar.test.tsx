// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { t } from '../i18n'
import { countProfiles } from '../lib/health'
import { tagCounts } from '../lib/tags'
import { Sidebar } from './sidebar'

afterEach(cleanup)

const base = { state: 'stopped', fpVerdict: 'match', fpDiff: [] as string[], proxyExpiry: '', lastOpenAt: '2026-10-13T10:00:00Z', cdpPort: 0 }
const rows = [
  { ...base, fpVerdict: 'drift', fpDiff: ['canvas'], tags: ['shop-us'] },
  { ...base, proxyExpiry: 'expired', tags: ['shop-us', 'vn'] },
  { ...base, tags: [] },
]

const sidebar = (onNavigate = vi.fn(), many = 0) => render(
  <Sidebar page="profiles" view="all" counts={countProfiles(rows)}
    tags={many ? Array.from({ length: many }, (_, i) => [`tag-${i}`, 1] as [string, number]) : tagCounts(rows)}
    sites={[]} siteCounts={new Map()} proxyCount={0} engineLabel="" engineReady onNavigate={onNavigate} onNewProfile={vi.fn()} />,
)

describe('Sidebar', () => {
  it('leaves Running and Needs attention to the status bar', () => {
    sidebar()
    expect(screen.getByRole('button', { name: new RegExp(t.nav.all) }).textContent).toContain('3')
    expect(screen.queryByRole('button', { name: new RegExp(t.nav.running) })).toBeNull()
    expect(screen.queryByRole('button', { name: new RegExp(t.nav.attention) })).toBeNull()
  })

  it('lists tags with counts and opens one', () => {
    const onNavigate = vi.fn()
    sidebar(onNavigate)
    const shop = screen.getByRole('button', { name: /shop-us/ })
    expect(shop.textContent).toContain('2')
    fireEvent.click(shop)
    expect(onNavigate).toHaveBeenCalledWith('profiles', 'tag:shop-us')
  })

  it('shows twelve tags until asked for all', () => {
    sidebar(vi.fn(), 15)
    expect(screen.queryByRole('button', { name: /tag-14/ })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: t.nav.showAll(15) }))
    expect(screen.getByRole('button', { name: /tag-14/ })).toBeTruthy()
  })
})
