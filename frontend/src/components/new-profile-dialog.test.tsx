// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { t } from '../i18n'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { service, site } from '../api'

const sites = [
  site.Site.createFrom({ id: 'tiktok', name: 'TikTok', group: 'video', domains: ['tiktok.com'], startUrl: 'https://www.tiktok.com/tiktokstudio', icon: 'tiktok', color: '#000000', custom: false }),
  site.Site.createFrom({ id: 'etsy', name: 'Etsy', group: 'commerce', domains: ['etsy.com'], startUrl: 'https://www.etsy.com/your/shops/me/dashboard', icon: 'etsy', color: '#F16521', custom: false }),
  site.Site.createFrom({ id: 'other', name: 'Other', group: 'other', domains: [], startUrl: '', icon: '', color: '#6B7280', custom: false }),
]

const createProfile = vi.fn().mockResolvedValue({})
const createProfiles = vi.fn().mockResolvedValue([])
vi.mock('../api', async (orig) => ({
  ...(await orig<typeof import('../api')>()),
  Api: {
    CreateProfile: (...a: unknown[]) => createProfile(...a),
    CreateProfiles: (...a: unknown[]) => createProfiles(...a),
    ListProxies: () => Promise.resolve([]),
    Engines: () => Promise.resolve([
      { engine: 'apostate', version: '0.5.1', chromium: '155', installed: true, problem: '', default: true },
      { engine: 'fury', version: '0.3.5', chromium: '155', installed: true, problem: '', default: false },
    ]),
    Sites: () => Promise.resolve(sites),
  },
}))
const { NewProfileDialog } = await import('./new-profile-dialog')

const defaults = service.Defaults.createFrom({ site: 'other', proxy: 'auto' })
const dialog = (over: Partial<Parameters<typeof NewProfileDialog>[0]> = {}) =>
  render(<NewProfileDialog defaultSite="tiktok" defaults={defaults} sites={sites} recent={[]} onClose={vi.fn()} onCreated={vi.fn()} {...over} />)

afterEach(() => { cleanup(); createProfile.mockClear(); createProfiles.mockClear() })

describe('NewProfileDialog', () => {
  it('creates from a name and Enter, with an automatic proxy and the platform start page', async () => {
    const onCreated = vi.fn()
    dialog({ onCreated })
    const name = screen.getByPlaceholderText(t.create.namePlaceholder)
    fireEvent.change(name, { target: { value: '  Kênh mới  ' } })
    fireEvent.submit(name.closest('form')!)
    await waitFor(() => expect(onCreated).toHaveBeenCalled())
    expect(createProfile.mock.calls[0][0]).toMatchObject({
      name: 'Kênh mới', proxyId: 'auto', site: 'tiktok', startUrl: 'https://www.tiktok.com/tiktokstudio',
    })
  })

  it('switches the start page with the platform until the user edits it', () => {
    dialog()
    fireEvent.click(screen.getByRole('button', { name: 'Platform' }))
    fireEvent.click(screen.getByRole('option', { name: /Etsy/ }))
    expect(screen.getByDisplayValue('https://www.etsy.com/your/shops/me/dashboard')).toBeTruthy()
    fireEvent.change(screen.getByDisplayValue('https://www.etsy.com/your/shops/me/dashboard'), { target: { value: 'https://example.com' } })
    fireEvent.click(screen.getByRole('button', { name: 'Platform' }))
    fireEvent.click(screen.getByRole('option', { name: /TikTok/ }))
    expect(screen.getByDisplayValue('https://example.com')).toBeTruthy()
  })

  it('does not submit without a name and closes on Escape', () => {
    const onClose = vi.fn()
    dialog({ defaultSite: 'other', onClose })
    expect((screen.getByRole('button', { name: t.create.submit }) as HTMLButtonElement).disabled).toBe(true)
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(onClose).toHaveBeenCalled()
  })

  it('bulk-creates with a name prefix and proxy mode', async () => {
    const onCreated = vi.fn()
    dialog({ onCreated })
    const name = screen.getByPlaceholderText(t.create.namePlaceholder)
    fireEvent.change(name, { target: { value: 'Shop' } })
    const count = document.querySelector('input[type="number"]') as HTMLInputElement
    fireEvent.change(count, { target: { value: '3' } })
    fireEvent.click(screen.getByRole('button', { name: /No proxy/ }))
    fireEvent.submit(name.closest('form')!)
    await waitFor(() => expect(onCreated).toHaveBeenCalled())
    expect(createProfiles).toHaveBeenCalledWith(expect.objectContaining({ baseName: 'Shop', count: 3, proxy: 'none' }))
    expect(createProfile).not.toHaveBeenCalled()
  })

  it('sends the engine chosen under Advanced, and none by default', async () => {
    const onCreated = vi.fn()
    dialog({ onCreated })
    const name = screen.getByPlaceholderText(t.create.namePlaceholder)
    fireEvent.change(name, { target: { value: 'Shop' } })
    fireEvent.change(screen.getByRole('spinbutton'), { target: { value: '2' } })
    fireEvent.click(screen.getByRole('button', { name: t.engines.advanced }))
    const select = await screen.findByRole('combobox')
    await waitFor(() => expect(screen.getByRole('option', { name: 'Fury' })).toBeTruthy())
    fireEvent.change(select, { target: { value: 'fury' } })
    fireEvent.submit(name.closest('form')!)
    await waitFor(() => expect(onCreated).toHaveBeenCalled())
    expect(createProfiles.mock.calls[0][0]).toMatchObject({ engine: 'fury', baseName: 'Shop', count: 2 })
  })

  it('leaves the engine empty when Advanced stays closed', async () => {
    const onCreated = vi.fn()
    dialog({ onCreated })
    const name = screen.getByPlaceholderText(t.create.namePlaceholder)
    fireEvent.change(name, { target: { value: 'Shop' } })
    fireEvent.submit(name.closest('form')!)
    await waitFor(() => expect(onCreated).toHaveBeenCalled())
    expect(createProfile.mock.calls[0][0]).toMatchObject({ engine: '' })
  })
})
