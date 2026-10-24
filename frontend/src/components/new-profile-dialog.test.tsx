// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { t } from '../i18n'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { service, site } from '../api'

const sites = [
  site.Site.createFrom({ id: 'tiktok', name: 'TikTok', group: 'video', domains: ['tiktok.com'], startUrl: 'https://www.tiktok.com/tiktokstudio', icon: 'tiktok', color: '#000000', custom: false }),
  site.Site.createFrom({ id: 'etsy', name: 'Etsy', group: 'commerce', domains: ['etsy.com'], startUrl: 'https://www.etsy.com/your/shops/me/dashboard', icon: 'etsy', color: '#F16521', custom: false }),
  site.Site.createFrom({ id: 'other', name: 'Other', group: 'other', domains: [], startUrl: '', icon: '', color: '#6B7280', custom: false }),
]

const freeProxy = (id: string) => ({ id, label: `gw.example:${id}`, country: 'US', exitIp: '1.1.1.1', rotating: false, usedBy: [], assignable: true })
let proxyList: unknown[] = [freeProxy('10001')]
const createProfile = vi.fn().mockResolvedValue({})
const createProfiles = vi.fn().mockResolvedValue([])
vi.mock('../api', async (orig) => ({
  ...(await orig<typeof import('../api')>()),
  Api: {
    CreateProfile: (...a: unknown[]) => createProfile(...a),
    CreateProfiles: (...a: unknown[]) => createProfiles(...a),
    ListProxies: () => Promise.resolve(proxyList),
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
  render(<NewProfileDialog defaultSite="tiktok" defaults={defaults} sites={sites} onClose={vi.fn()} onCreated={vi.fn()} {...over} />)

afterEach(() => { cleanup(); createProfile.mockClear(); createProfiles.mockClear(); proxyList = [freeProxy('10001')]; localStorage.clear() })

describe('NewProfileDialog', () => {
  it('creates from a name and Enter, with an automatic proxy and the platform start page', async () => {
    const onCreated = vi.fn()
    dialog({ onCreated })
    const name = screen.getByPlaceholderText(t.create.namePlaceholder)
    fireEvent.change(name, { target: { value: '  Kênh mới  ' } })
    fireEvent.submit(name.closest('form')!)
    await waitFor(() => expect(onCreated).toHaveBeenCalled())
    expect(createProfile.mock.calls[0][0]).toMatchObject({
      name: 'Kênh mới', proxyId: 'auto', sites: ['tiktok'], startUrl: 'https://www.tiktok.com/tiktokstudio',
    })
  })

  it('adds and removes the start page of each platform, keeping typed pages, and sends every platform', async () => {
    const onCreated = vi.fn()
    dialog({ onCreated })
    const tiktok = 'https://www.tiktok.com/tiktokstudio', etsy = 'https://www.etsy.com/your/shops/me/dashboard'
    const urls = () => (screen.getByPlaceholderText(t.create.startUrlHint) as HTMLTextAreaElement).value
    expect(urls()).toBe(tiktok)
    fireEvent.change(screen.getByPlaceholderText(t.create.startUrlHint), { target: { value: `${tiktok}\nhttps://mine.example/` } })
    fireEvent.click(screen.getByRole('button', { name: t.sites.change }))
    fireEvent.click(screen.getByRole('option', { name: /Etsy/ }))
    fireEvent.click(screen.getByRole('button', { name: t.sites.done }))
    expect(urls()).toBe(`${tiktok}\nhttps://mine.example/\n${etsy}`)
    // Removing TikTok's chip drops its start page and keeps the typed one.
    fireEvent.click(screen.getByRole('button', { name: t.sites.remove('TikTok') }))
    expect(urls()).toBe(`https://mine.example/\n${etsy}`)
    const name = screen.getByPlaceholderText(t.create.namePlaceholder)
    fireEvent.change(name, { target: { value: 'Shop' } })
    fireEvent.submit(name.closest('form')!)
    await waitFor(() => expect(onCreated).toHaveBeenCalled())
    expect(createProfile.mock.calls[0][0]).toMatchObject({ sites: ['etsy'], startUrl: `https://mine.example/\n${etsy}` })
  })

  it('closes only the platform modal on Escape, and Enter there does not create the profile', () => {
    const onClose = vi.fn(), onCreated = vi.fn()
    dialog({ onClose, onCreated })
    fireEvent.change(screen.getByPlaceholderText(t.create.namePlaceholder), { target: { value: 'Shop' } })
    fireEvent.click(screen.getByRole('button', { name: t.sites.change }))
    fireEvent.submit(screen.getByRole('dialog', { name: t.sites.pick }))
    expect(createProfile).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: t.sites.change }))
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(screen.queryByRole('dialog', { name: t.sites.pick })).toBeNull()
    expect(screen.getByRole('dialog', { name: t.create.title })).toBeTruthy()
    expect(onClose).not.toHaveBeenCalled()
  })

  it('does not submit without a name and closes on Escape', () => {
    const onClose = vi.fn()
    dialog({ defaultSite: 'other', onClose })
    expect((screen.getByRole('button', { name: t.create.submit }) as HTMLButtonElement).disabled).toBe(true)
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(onClose).toHaveBeenCalled()
  })

  it("opens TikTok Shop's seller center for the chosen proxy's country", async () => {
    proxyList = [freeProxy('10001'), { ...freeProxy('10002'), country: 'VN' }]
    const shop = site.Site.createFrom({ id: 'tiktok-shop', name: 'TikTok Shop', group: 'commerce', domains: ['seller.tiktok.com'], startUrl: 'https://seller.tiktok.com',
      startUrls: { US: 'https://seller-us.tiktok.com', VN: 'https://seller-vn.tiktok.com' }, icon: 'tiktok', color: '#FE2C55', custom: false })
    dialog({ defaultSite: 'tiktok-shop', sites: [...sites, shop] })
    const urls = () => (screen.getByPlaceholderText(t.create.startUrlHint) as HTMLTextAreaElement).value
    const proxy = await screen.findByRole('combobox', { name: t.create.proxy })
    await screen.findByRole('option', { name: /gw.example:10002/ })
    fireEvent.change(proxy, { target: { value: '10001' } })
    await waitFor(() => expect(urls()).toBe('https://seller-us.tiktok.com'))
    fireEvent.change(proxy, { target: { value: '10002' } })
    await waitFor(() => expect(urls()).toBe('https://seller-vn.tiktok.com'))
  })

  it('asks how many only when creating several profiles', () => {
    dialog()
    expect(screen.queryByRole('spinbutton')).toBeNull()
    expect(screen.getByRole('dialog', { name: t.create.title })).toBeTruthy()
    cleanup()
    dialog({ initialCount: 5 })
    expect((screen.getByRole('spinbutton') as HTMLInputElement).value).toBe('5')
    expect(screen.getByRole('dialog', { name: t.palette.createSeveral })).toBeTruthy()
  })

  it('bulk-creates with a name prefix and proxy mode', async () => {
    const onCreated = vi.fn()
    dialog({ onCreated, initialCount: 5 })
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
    dialog({ onCreated, initialCount: 5 })
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

  it('starts on No proxy when Auto is the default but no proxy is free, and keeps Auto remembered', async () => {
    proxyList = [{ ...freeProxy('10001'), usedBy: ['One'], assignable: false }]
    localStorage.setItem('anyprofile.lastProxy', 'auto')
    const onCreated = vi.fn()
    dialog({ onCreated })
    const name = screen.getByPlaceholderText(t.create.namePlaceholder)
    fireEvent.change(name, { target: { value: 'Shop' } })
    await waitFor(() => expect((screen.getByRole('combobox', { name: t.create.proxy }) as HTMLSelectElement).value).toBe(''))
    expect(screen.getByText(t.create.realIpWarning)).toBeTruthy()
    // The warning leads to buying one, in a dialog over this one.
    fireEvent.click(screen.getByRole('button', { name: t.proxies.buy }))
    const buy = screen.getByRole('dialog', { name: t.proxies.buy })
    fireEvent.click(within(buy).getAllByRole('button', { name: t.common.close })[0])
    expect(screen.queryByRole('dialog', { name: t.proxies.buy })).toBeNull()
    expect((screen.getByRole('button', { name: t.create.submit }) as HTMLButtonElement).disabled).toBe(false)
    fireEvent.submit(name.closest('form')!)
    await waitFor(() => expect(onCreated).toHaveBeenCalled())
    expect(createProfile.mock.calls[0][0]).toMatchObject({ proxyId: '' })
    expect(localStorage.getItem('anyprofile.lastProxy')).toBe('auto')
  })

  it('creates several on No proxy when too few proxies are free', async () => {
    proxyList = [freeProxy('10001')]
    const onCreated = vi.fn()
    dialog({ onCreated, initialCount: 3 })
    const name = screen.getByPlaceholderText(t.create.namePlaceholder)
    fireEvent.change(name, { target: { value: 'Shop' } })
    await waitFor(() => expect(screen.getByRole('button', { name: t.duplicate.proxyNone }).getAttribute('aria-pressed')).toBe('true'))
    fireEvent.submit(name.closest('form')!)
    await waitFor(() => expect(onCreated).toHaveBeenCalled())
    expect(createProfiles.mock.calls[0][0]).toMatchObject({ count: 3, proxy: 'none' })
  })

  it('warns about the real IP when No proxy is chosen', async () => {
    dialog()
    await waitFor(() => expect(screen.getByRole('option', { name: t.create.proxyAutoFree(1) })).toBeTruthy())
    expect(screen.queryByText(t.create.realIpWarning)).toBeNull()
    fireEvent.change(screen.getByRole('combobox', { name: t.create.proxy }), { target: { value: '' } })
    expect(screen.getByText(t.create.realIpWarning)).toBeTruthy()
  })

  it('counts free proxies in the Auto option', async () => {
    proxyList = [freeProxy('10001'), freeProxy('10002')]
    dialog()
    const auto = await screen.findByRole('option', { name: t.create.proxyAutoFree(2) })
    expect(auto.textContent).toContain('2')
  })

  it('offers the proxy chosen for the last profile, and warns that it is shared', async () => {
    proxyList = [freeProxy('10001'), freeProxy('10002')]
    const first = dialog({ onCreated: vi.fn() })
    const name = screen.getByPlaceholderText(t.create.namePlaceholder)
    fireEvent.change(name, { target: { value: 'One' } })
    await screen.findByRole('option', { name: /gw.example:10002/ })
    fireEvent.change(screen.getByRole('combobox', { name: t.create.proxy }), { target: { value: '10002' } })
    fireEvent.submit(name.closest('form')!)
    await waitFor(() => expect(createProfile).toHaveBeenCalled())
    expect(createProfile.mock.calls[0][0]).toMatchObject({ proxyId: '10002' })
    first.unmount()

    proxyList = [freeProxy('10001'), { ...freeProxy('10002'), usedBy: ['One'], assignable: false }]
    dialog()
    // Sharing a proxy is a choice the user may make: no warning, the picker
    // only names the other profile.
    await screen.findByRole('option', { name: /gw.example:10002.*\(One\)/ })
    expect((screen.getByRole('combobox', { name: t.create.proxy }) as HTMLSelectElement).value).toBe('10002')
    expect(document.querySelector('.realip')).toBeNull()
  })

  it('falls back to the default when the remembered proxy is gone, and never remembers No proxy', async () => {
    localStorage.setItem('anyprofile.lastProxy', 'deleted')
    const onCreated = vi.fn()
    dialog({ onCreated })
    await screen.findByRole('option', { name: t.create.proxyAutoFree(1) })
    const select = screen.getByRole('combobox', { name: t.create.proxy }) as HTMLSelectElement
    expect(select.value).toBe('auto')
    fireEvent.change(select, { target: { value: '' } })
    const name = screen.getByPlaceholderText(t.create.namePlaceholder)
    fireEvent.change(name, { target: { value: 'Direct' } })
    fireEvent.submit(name.closest('form')!)
    await waitFor(() => expect(onCreated).toHaveBeenCalled())
    expect(localStorage.getItem('anyprofile.lastProxy')).toBeNull()
  })
})
