// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { t } from '../i18n'
import { countProfiles } from '../lib/health'
import { tagCounts } from '../lib/tags'
import { Sidebar } from './sidebar'

afterEach(() => { cleanup(); localStorage.clear() })

const base = { state: 'stopped', fpVerdict: 'match', fpDiff: [] as string[], proxyExpiry: '', lastOpenAt: '2026-10-13T10:00:00Z', cdpPort: 0 }
const rows = [
  { ...base, fpVerdict: 'drift', fpDiff: ['canvas'], tags: ['shop-us'] },
  { ...base, proxyExpiry: 'expired', tags: ['shop-us', 'vn'] },
  { ...base, tags: [] },
]

const sidebar = (onNavigate = vi.fn(), many = 0, onOpenTools = vi.fn(), proActive = false) => render(
  <Sidebar page="profiles" view="all" counts={countProfiles(rows)}
    tags={many ? Array.from({ length: many }, (_, i) => [`tag-${i}`, 1] as [string, number]) : tagCounts(rows)}
    sites={[]} siteCounts={new Map()} proxyCount={0} mobileCount={2} engineLabel="" engineReady
    pro={proActive ? ({ active: true, kind: 'lifetime' } as never) : undefined}
    onNavigate={onNavigate} onNewProfile={vi.fn()} onOpenTools={onOpenTools} />,
)

describe('Sidebar', () => {
  it('leaves Running and Needs attention to the status bar', () => {
    sidebar()
    expect(screen.getByRole('button', { name: new RegExp(t.nav.all) }).textContent).toContain('3')
    expect(screen.queryByRole('button', { name: new RegExp(t.nav.running) })).toBeNull()
    expect(screen.queryByRole('button', { name: new RegExp(t.nav.attention) })).toBeNull()
  })

  it('has Web and Mobile, Mobile with its device count', () => {
    const onNavigate = vi.fn()
    sidebar(onNavigate)
    expect(screen.getByRole('button', { name: new RegExp(t.nav.all) })).toBeTruthy()
    const mobile = screen.getByRole('button', { name: new RegExp(t.nav.mobile) })
    expect(mobile.textContent).toContain('2')
    fireEvent.click(mobile)
    expect(onNavigate).toHaveBeenCalledWith('mobile')
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

  it('opens Buy proxies and Creator tools from the footer', () => {
    const onNavigate = vi.fn()
    const onOpenTools = vi.fn()
    sidebar(onNavigate, 0, onOpenTools)
    fireEvent.click(screen.getByRole('button', { name: t.proxies.buy }))
    expect(screen.getByRole('dialog', { name: t.proxies.buy })).toBeTruthy()
    fireEvent.keyDown(window, { key: 'Escape' })
    fireEvent.click(screen.getByRole('button', { name: t.nav.tools }))
    expect(onOpenTools).toHaveBeenCalledOnce()
    expect(onNavigate).not.toHaveBeenCalled()
  })

  it('shows eight platforms until asked for all, and collapses them', () => {
    const sites = Array.from({ length: 10 }, (_, i) => ({
      id: `site-${i}`, name: `Platform ${String(i).padStart(2, '0')}`, domains: [], startUrl: '', color: '#000',
      group: 'other' as const, icon: '', custom: false,
    }))
    const siteCounts = new Map(sites.map((s) => [s.id, 1]))
    render(
      <Sidebar page="profiles" view="all" counts={countProfiles(rows)}
        tags={[]} sites={sites} siteCounts={siteCounts} proxyCount={0} mobileCount={0} engineLabel="" engineReady
        onNavigate={vi.fn()} onNewProfile={vi.fn()} onOpenTools={vi.fn()} />,
    )
    expect(screen.queryByRole('button', { name: /Platform 09/ })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: t.nav.showAll(10) }))
    expect(screen.getByRole('button', { name: /Platform 09/ })).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: t.nav.sites }))
    expect(screen.queryByRole('button', { name: /Platform 00/ })).toBeNull()
  })

  it('restores the open or collapsed state of each section independently', () => {
    const sites = [{ id: 'site-1', name: 'Example', domains: [], startUrl: '', color: '#000', group: 'other' as const, icon: '', custom: false }]
    const mount = () => render(<Sidebar page="profiles" view="all" counts={countProfiles(rows)}
      tags={tagCounts(rows)} sites={sites} siteCounts={new Map([['site-1', 1]])} proxyCount={0} mobileCount={0}
      engineLabel="" engineReady onNavigate={vi.fn()} onNewProfile={vi.fn()} onOpenTools={vi.fn()} />)
    const section = (name: string) => screen.getByRole('button', { name })

    mount()
    fireEvent.click(section(t.nav.sites))
    fireEvent.click(section(t.nav.tags))
    cleanup()
    mount()
    expect(section(t.nav.sites).getAttribute('aria-expanded')).toBe('false')
    expect(section(t.nav.tags).getAttribute('aria-expanded')).toBe('false')
    fireEvent.click(section(t.nav.sites))
    cleanup()
    mount()
    expect(section(t.nav.sites).getAttribute('aria-expanded')).toBe('true')
    expect(section(t.nav.tags).getAttribute('aria-expanded')).toBe('false')
  })

  it('shows the Pro mark next to AnyProfile only when Pro is active', () => {
    const { container } = sidebar(vi.fn(), 0, vi.fn(), false)
    expect(container.querySelector('.brand-tier')).toBeNull()
    expect(container.querySelector('.prostatus')).toBeNull()
    cleanup()
    const { container: proContainer } = sidebar(vi.fn(), 0, vi.fn(), true)
    expect(proContainer.querySelector('.brand-tier')?.textContent).toBe('PRO')
    expect(proContainer.querySelector('.prostatus')).toBeNull()
  })
})
