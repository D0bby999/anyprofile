// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { t } from '../i18n'
import { service, site } from '../api'
import type { View } from '../components/sidebar'
import { ToastProvider } from '../components/toast'

const arrangeWindows = vi.fn(() => Promise.resolve(2))
const setProfileOrder = vi.fn((_: string[]) => Promise.resolve())
vi.mock('../api', async (orig) => ({
  ...(await orig<typeof import('../api')>()),
  onDataChanged: () => () => {},
  Api: { ListProxies: () => Promise.resolve([]), UpdateStatus: () => Promise.resolve(null), ArrangeWindows: () => arrangeWindows(), SetProfileOrder: (ids: string[]) => setProfileOrder(ids) },
}))
const { ProfilesPage } = await import('./profiles-page')

const sites = [site.Site.createFrom({ id: 'other', name: 'Other', group: 'other', domains: [], startUrl: '', icon: '', color: '#6B7280', custom: false })]
const rows = [service.ProfileRow.createFrom({
  id: 'a', name: 'Kênh A', sites: ['other'], serial: 1, engine: 'apostate', personaId: 'apostate-macos', machine: '', proxyId: '',
  state: 'stopped', cdpPort: 0, tags: ['shop-us'], notes: '', login: '', lastOpenAt: '', fpVerdict: '', fpDiff: [], pinned: false,
})]
const page = (view: View = 'all', onClearView = vi.fn(), list = rows) => render(<ToastProvider>
  <ProfilesPage rows={list} sites={sites} view={view} engineReady fileVaultOn error="" setError={vi.fn()}
    reload={vi.fn()} onNew={vi.fn()} onNeedEngine={vi.fn()} onClearView={onClearView} onView={vi.fn()} onOpenSettings={vi.fn()} activeId={null} setActiveId={vi.fn()} />
</ToastProvider>)

afterEach(() => { cleanup(); localStorage.clear() })

describe('profiles toolbar', () => {
  it('offers to arrange windows once two profiles are open', () => {
    page()
    expect(screen.queryByRole('button', { name: t.profiles.arrange })).toBeNull()
    cleanup()
    const open = (id: string) => service.ProfileRow.createFrom({ ...rows[0], id, name: id, state: 'running' })
    page('all', vi.fn(), [open('a'), open('b')])
    fireEvent.click(screen.getByRole('button', { name: t.profiles.arrange }))
    expect(arrangeWindows).toHaveBeenCalledOnce()
  })

  it('moves a row with the keyboard and keeps the order, also for rows a filter hides', () => {
    const mk = (id: string, serial: number, tags: string[] = ['shop-us']) => service.ProfileRow.createFrom({ ...rows[0], id, name: id, serial, tags })
    page('tag:shop-us', vi.fn(), [mk('a', 1), mk('h', 2, []), mk('b', 3), mk('c', 4)])
    const names = () => [...document.querySelectorAll('td.cellname b')].map((b) => b.textContent)
    expect(names()).toEqual(['a', 'b', 'c'])
    fireEvent.keyDown(screen.getByRole('button', { name: t.profiles.dragRow('c') }), { key: 'ArrowUp' })
    expect(names()).toEqual(['a', 'c', 'b'])
    // h, hidden by the filter, keeps its place between a and the rest.
    expect(setProfileOrder).toHaveBeenLastCalledWith(['a', 'h', 'c', 'b'])
    // Sorted by name, rows cannot be dragged: where they land would not stay.
    // Trying says so, and offers the # order back.
    fireEvent.click(screen.getByRole('columnheader', { name: t.profiles.colName }))
    expect(screen.queryByRole('button', { name: t.profiles.dragRow('c') })).toBeNull()
    const cName = [...document.querySelectorAll<HTMLElement>('td.cellname b')].find((b) => b.textContent === 'c')!
    fireEvent.pointerDown(cName, { button: 0, clientX: 10, clientY: 100 })
    fireEvent.pointerMove(window, { clientX: 10, clientY: 140 })
    fireEvent.pointerUp(window)
    expect(screen.getByText(t.profiles.dragNeedsNo)).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: t.profiles.sortByNo }))
    expect(screen.getByRole('button', { name: t.profiles.dragRow('c') })).toBeTruthy()
  })

  it('hides a column and switches density, and remembers both', () => {
    page()
    expect(screen.getByRole('columnheader', { name: t.profiles.colProxy })).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: t.view.button }))
    fireEvent.click(screen.getByRole('menuitemcheckbox', { name: t.view.cols.proxy }))
    fireEvent.click(screen.getByRole('menuitemradio', { name: t.view.compact }))
    expect(screen.queryByRole('columnheader', { name: t.profiles.colProxy })).toBeNull()
    expect(screen.getByRole('table').className).toContain('compact')
    cleanup()
    page()
    expect(screen.queryByRole('columnheader', { name: t.profiles.colProxy })).toBeNull()
    expect(screen.getByRole('table').className).toContain('compact')
  })

  it('shows the active filter as a chip that clears it', () => {
    const onClearView = vi.fn()
    page('tag:shop-us', onClearView)
    const label = t.filters.tag('shop-us')
    expect(screen.getByText(label)).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: t.filters.remove(label) }))
    expect(onClearView).toHaveBeenCalled()
  })

  it('finds a profile by #number and ignores diacritics', () => {
    page()
    const search = screen.getByRole('textbox', { name: t.profiles.search })
    fireEvent.change(search, { target: { value: 'kenh a' } })
    expect(screen.getByText('Kênh A')).toBeTruthy()
    fireEvent.change(search, { target: { value: '#2' } })
    expect(screen.queryByText('Kênh A')).toBeNull()
  })
})
