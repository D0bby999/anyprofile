// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { t } from '../i18n'
import { service, site } from '../api'

const updateProfile = vi.fn().mockResolvedValue(undefined)
const trashProfile = vi.fn().mockResolvedValue(undefined)
vi.mock('../api', async (orig) => ({
  ...(await orig<typeof import('../api')>()),
  Api: {
    ListProxies: () => Promise.resolve([
      { id: 'px', label: 'gw.example:10001', country: 'US', exitIp: '1.1.1.1', rotating: false, usedBy: ['Kênh A'] },
      { id: 'py', label: 'gw.example:10002', country: 'DE', exitIp: '2.2.2.2', rotating: false, usedBy: [] },
    ]),
    UpdateProfile: (...a: unknown[]) => updateProfile(...a),
    TrashProfile: (...a: unknown[]) => trashProfile(...a),
    StartProfile: () => Promise.resolve(),
    StopProfile: () => Promise.resolve(),
    TOTPCode: () => Promise.resolve({ code: '000000', secondsLeft: 30 }),
  },
}))
const { ProfilesPage } = await import('../pages/profiles-page')
const { ConfirmProvider } = await import('./confirm')

const sites = [
  site.Site.createFrom({ id: 'tiktok', name: 'TikTok', group: 'video', domains: ['tiktok.com'], startUrl: '', icon: 'tiktok', color: '#000000', custom: false }),
  site.Site.createFrom({ id: 'other', name: 'Other', group: 'other', domains: [], startUrl: '', icon: '', color: '#6B7280', custom: false }),
]
const row = (id: string, name: string, serial: number) => service.ProfileRow.createFrom({
  id, name, site: 'tiktok', serial, engine: 'apostate', engineVersion: '0.5.1', personaId: 'apostate-macos', machine: 'Apple M3 Pro · 2560×1440',
  proxyId: 'px', proxyLabel: 'gw.example:10001', proxyCountry: 'US', proxyExitIp: '1.1.1.1', timezone: 'America/New_York', languages: 'en-US,en',
  state: 'stopped', stateError: '', cdpPort: 0, tags: ['shop'], notes: '', startUrl: '', login: '', seedHex: '00000000000a11ce',
  lastOpenAt: '', createdAt: '2026-10-12T00:00:00Z', fpVerdict: '', fpDiff: [],
})
const rows = [row('a', 'Kênh A', 1), row('b', 'Kênh B', 2), row('c', 'Kênh C', 3)]

const page = () => render(
  <ConfirmProvider>
    <ProfilesPage rows={rows} sites={sites} recent={[]} view="all" engineReady fileVaultOn error="" setError={vi.fn()}
      reload={vi.fn()} onNew={vi.fn()} onNeedEngine={vi.fn()} onClearView={vi.fn()} />
  </ConfirmProvider>,
)
const drawer = () => screen.queryByRole('complementary', { name: /^Details of/ })
const openRow = (name: string) => fireEvent.click(screen.getByText(name), { detail: 1 })

afterEach(() => { cleanup(); vi.useRealTimers(); updateProfile.mockClear(); trashProfile.mockClear() })

describe('profile drawer', () => {
  it('opens on a row click, moves with the arrows and closes on Escape', () => {
    page()
    openRow('Kênh A')
    expect(drawer()?.getAttribute('aria-label')).toBe(t.drawer.label('Kênh A'))
    fireEvent.keyDown(window, { key: 'ArrowDown' })
    expect(drawer()?.getAttribute('aria-label')).toBe(t.drawer.label('Kênh B'))
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(drawer()).toBeNull()
  })

  it('does not open from the row checkbox', () => {
    page()
    fireEvent.click(screen.getByRole('checkbox', { name: 'Kênh B' }))
    expect(drawer()).toBeNull()
  })

  it('saves notes once, after typing stops', async () => {
    page()
    openRow('Kênh A')
    vi.useFakeTimers()
    const notes = within(drawer()!).getByRole('textbox', { name: t.drawer.notes })
    fireEvent.change(notes, { target: { value: 'n' } })
    fireEvent.change(notes, { target: { value: 'new note' } })
    await act(async () => { vi.advanceTimersByTime(700) })
    expect(updateProfile).toHaveBeenCalledTimes(1)
    expect(updateProfile.mock.calls[0][0]).toMatchObject({ id: 'a', notes: 'new note' })
  })

  it('renames with Enter and keeps the name on Escape', () => {
    page()
    openRow('Kênh A')
    const d = within(drawer()!)
    fireEvent.click(d.getByTitle(t.drawer.rename))
    fireEvent.change(d.getByRole('textbox', { name: t.drawer.rename }), { target: { value: 'Shop A' } })
    fireEvent.keyDown(d.getByRole('textbox', { name: t.drawer.rename }), { key: 'Enter' })
    expect(updateProfile.mock.calls[0][0]).toMatchObject({ id: 'a', name: 'Shop A' })
    fireEvent.click(d.getByTitle(t.drawer.rename))
    fireEvent.change(d.getByRole('textbox', { name: t.drawer.rename }), { target: { value: 'Nope' } })
    fireEvent.keyDown(d.getByRole('textbox', { name: t.drawer.rename }), { key: 'Escape' })
    expect(updateProfile).toHaveBeenCalledTimes(1)
    expect(drawer()).not.toBeNull()
    expect(d.getByTitle(t.drawer.rename).textContent).toBe('Kênh A')
  })

  it('applies a new proxy only on Apply', async () => {
    page()
    openRow('Kênh A')
    const d = within(drawer()!)
    await waitFor(() => expect(d.getByRole('option', { name: /gw.example:10002/ })).toBeTruthy())
    fireEvent.change(d.getByRole('combobox', { name: t.create.proxy }), { target: { value: 'py' } })
    expect(updateProfile).not.toHaveBeenCalled()
    fireEvent.click(d.getByRole('button', { name: t.drawer.apply }))
    await waitFor(() => expect(updateProfile).toHaveBeenCalled())
    expect(updateProfile.mock.calls[0][0]).toMatchObject({ id: 'a', proxyId: 'py' })
  })

  it('moves the profile to the trash after confirming', async () => {
    page()
    openRow('Kênh A')
    const d = within(drawer()!)
    fireEvent.click(d.getByRole('button', { name: t.drawer.advanced }))
    fireEvent.click(d.getByRole('button', { name: t.drawer.moveToTrash }))
    const dialog = await screen.findByRole('dialog')
    fireEvent.click(within(dialog).getByRole('button', { name: t.confirm.trash('Kênh A').confirm }))
    await waitFor(() => expect(trashProfile).toHaveBeenCalledWith('a'))
  })
})
