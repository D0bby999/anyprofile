// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { t } from '../i18n'
import { service, site } from '../api'
import type { View } from '../components/sidebar'

const arrangeWindows = vi.fn(() => Promise.resolve(2))
vi.mock('../api', async (orig) => ({
  ...(await orig<typeof import('../api')>()),
  onDataChanged: () => () => {},
  Api: { ListProxies: () => Promise.resolve([]), UpdateStatus: () => Promise.resolve(null), ArrangeWindows: () => arrangeWindows() },
}))
const { ProfilesPage } = await import('./profiles-page')

const sites = [site.Site.createFrom({ id: 'other', name: 'Other', group: 'other', domains: [], startUrl: '', icon: '', color: '#6B7280', custom: false })]
const rows = [service.ProfileRow.createFrom({
  id: 'a', name: 'Kênh A', sites: ['other'], serial: 1, engine: 'apostate', personaId: 'apostate-macos', machine: '', proxyId: '',
  state: 'stopped', cdpPort: 0, tags: ['shop-us'], notes: '', login: '', lastOpenAt: '', fpVerdict: '', fpDiff: [], pinned: false,
})]
const page = (view: View = 'all', onClearView = vi.fn(), list = rows) => render(
  <ProfilesPage rows={list} sites={sites} view={view} engineReady fileVaultOn error="" setError={vi.fn()}
    reload={vi.fn()} onNew={vi.fn()} onNeedEngine={vi.fn()} onClearView={onClearView} onView={vi.fn()} onOpenSettings={vi.fn()} activeId={null} setActiveId={vi.fn()} />,
)

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
