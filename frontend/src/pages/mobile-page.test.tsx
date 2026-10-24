// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { t } from '../i18n'
import { ConfirmProvider } from '../components/confirm'
import { ToastProvider } from '../components/toast'

const api = {
  ListProxies: vi.fn(() => Promise.resolve([{ id: 'px', label: 'gw.example:1', country: 'JP', exitIp: '2.2.2.2', rotating: false, usedBy: [], assignable: true }])),
  StartMobileDevice: vi.fn(() => Promise.resolve({ warning: '' })),
  BackupMobileDevice: vi.fn(() => Promise.resolve()),
  InstallMobileApps: vi.fn(() => Promise.resolve(2)),
  ShowMobileDevice: vi.fn(() => Promise.resolve()),
  InstallMobileRuntime: vi.fn(() => Promise.resolve()),
  CreateMobileDevice: vi.fn(() => Promise.resolve({ id: 'n' })),
  TrashMobileDevice: vi.fn(() => Promise.resolve()),
  RestoreMobileDevice: vi.fn(() => Promise.resolve()),
}
const openExternal = vi.fn()
vi.mock('../api', async (orig) => ({
  ...(await orig<typeof import('../api')>()),
  openExternal: (u: string) => openExternal(u),
  onDataChanged: () => () => {},
  onEngineProgress: () => () => {},
  Api: api,
}))
const { MobilePage } = await import('./mobile-page')

afterEach(() => { cleanup(); vi.clearAllMocks() })

const device = (extra = {}) => ({
  id: 'd1', name: 'Phone 1', driver: 'vm', platform: 'darwin', driverRef: 'd1', proxyId: 'px', ramMb: 3072, cpus: 4,
  imageVersion: '', notes: '', tags: ['jp'], pinned: false, serial: 1, createdAt: '', updatedAt: '', lastOpenAt: '',
  backedUpAt: '', deletedAt: '', state: 'stopped', error: '', ...extra,
})
const avail = (extra = {}) => ({ supported: true, driver: 'vm', pro: true, ready: true, reason: '', message: '', ...extra })

function page(rows: object[], a: object, onOpenPro = vi.fn()) {
  const mobile = { rows, avail: a, error: '', setError: vi.fn(), reload: vi.fn() } as never
  render(<ConfirmProvider><ToastProvider><MobilePage mobile={mobile} onOpenPro={onOpenPro} onOpenSettings={vi.fn()} /></ToastProvider></ConfirmProvider>)
  return onOpenPro
}

describe('MobilePage', () => {
  it('explains Pro, honestly, and cannot create without it', () => {
    const onOpenPro = page([], avail({ pro: false }))
    expect(screen.getByText(t.mobile.proText)).toBeTruthy()
    expect(screen.getByText(t.mobile.proHonest)).toBeTruthy()
    expect((screen.getByRole('button', { name: t.mobile.newDevice }) as HTMLButtonElement).disabled).toBe(true)
    fireEvent.click(screen.getByRole('button', { name: t.mobile.getPro }))
    expect(onOpenPro).toHaveBeenCalled()
  })

  it('keeps a lapsed subscription\u2019s devices: no opening, backing up still works', async () => {
    page([device()], avail({ pro: false }))
    expect(screen.getByText(t.mobile.proLapsed)).toBeTruthy()
    expect((screen.getByRole('button', { name: t.mobile.open }) as HTMLButtonElement).disabled).toBe(true)
    fireEvent.click(screen.getByRole('button', { name: t.mobile.backup }))
    await waitFor(() => expect(api.BackupMobileDevice).toHaveBeenCalledWith('d1'))
  })

  it('downloads Android when the runtime is missing', async () => {
    page([], avail({ ready: false, reason: 'android_runtime_missing', message: '' }))
    fireEvent.click(screen.getByRole('button', { name: t.mobile.install }))
    await waitFor(() => expect(api.InstallMobileRuntime).toHaveBeenCalled())
  })

  it('says what the driver is missing', () => {
    page([], avail({ ready: false, reason: 'ldplayer_missing', message: 'Install LDPlayer 9.' }))
    expect(screen.getByRole('status').textContent).toContain(t.errors.ldplayer_missing)
    expect((screen.getByRole('button', { name: t.mobile.newDevice }) as HTMLButtonElement).disabled).toBe(true)
  })

  it('opens a device and warns when memory is tight', async () => {
    api.StartMobileDevice.mockResolvedValueOnce({ warning: 'ram_tight' })
    page([device()], avail())
    expect(await screen.findByText('gw.example:1')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: t.mobile.open }))
    await waitFor(() => expect(api.StartMobileDevice).toHaveBeenCalledWith('d1'))
    expect(await screen.findByText(t.mobile.ramTight)).toBeTruthy()
  })

  it('says a running device has no network when its proxy stopped answering', async () => {
    api.ListProxies.mockImplementationOnce(() => Promise.resolve([{ id: 'px', label: 'gw.example:1', country: 'JP', exitIp: '2.2.2.2',
      rotating: false, usedBy: [], assignable: false, failedAt: '2026-10-24T08:00:00Z' }]))
    page([device({ proxyId: 'px', state: 'running' })], avail())
    expect(await screen.findByText(t.mobile.proxyDown)).toBeTruthy()
  })

  it('moves a device to the trash with Undo', async () => {
    page([device()], avail())
    fireEvent.click(screen.getByRole('button', { name: t.mobile.trash }))
    await waitFor(() => expect(api.TrashMobileDevice).toHaveBeenCalledWith('d1'))
    fireEvent.click(await screen.findByRole('button', { name: t.common.undo }))
    await waitFor(() => expect(api.RestoreMobileDevice).toHaveBeenCalledWith('d1'))
  })

  it('says Beta, and Coming soon instead of a Download that can only fail while the runtime is not out', () => {
    const missing = { ready: false, reason: 'android_runtime_missing' }
    const comp = (latest: string) => ({ runtime: { engine: 'android-vm', installed: '', latest }, image: { engine: 'android-vm-image', installed: '', latest: '' } })
    page([], avail({ ...missing, install: comp('') }))
    expect(screen.getByText(t.mobile.beta)).toBeTruthy()
    expect(screen.getByText(t.mobile.comingSoon)).toBeTruthy()
    expect(screen.queryByRole('button', { name: t.mobile.install })).toBeNull()
    cleanup()
    page([], avail({ ...missing, install: comp('1.0.0') }))
    expect(screen.getByText(t.mobile.installText)).toBeTruthy()
    expect(screen.getByRole('button', { name: t.mobile.install })).toBeTruthy()
  })

  it('links to the guide for installing apps', () => {
    page([device()], avail())
    fireEvent.click(screen.getByRole('link', { name: t.mobile.apkGuide }))
    expect(openExternal).toHaveBeenCalledWith('https://github.com/D0bby999/anyprofile/blob/master/docs/mobile-apps.md')
  })

  it('installs apps on a running device only', async () => {
    page([device({ state: 'running' })], avail())
    fireEvent.click(screen.getByRole('button', { name: t.mobile.installApps }))
    await waitFor(() => expect(api.InstallMobileApps).toHaveBeenCalledWith('d1'))
    expect(await screen.findByText(t.mobile.appsInstalled(2))).toBeTruthy()
    cleanup()
    page([device()], avail())
    expect(screen.queryByRole('button', { name: t.mobile.installApps })).toBeNull()
  })

  it('shows Close and Show window for a running device and locks backup', async () => {
    page([device({ state: 'running' })], avail())
    expect(screen.getByRole('button', { name: t.mobile.close })).toBeTruthy()
    expect((screen.getByRole('button', { name: t.mobile.backup }) as HTMLButtonElement).disabled).toBe(true)
    fireEvent.click(screen.getByRole('button', { name: t.mobile.show }))
    await waitFor(() => expect(api.ShowMobileDevice).toHaveBeenCalledWith('d1'))
  })

  it('creates a device with the default memory and an automatic proxy', async () => {
    page([], avail())
    fireEvent.click(screen.getAllByRole('button', { name: t.mobile.newDevice })[0])
    fireEvent.change(screen.getByPlaceholderText(t.mobile.namePlaceholder), { target: { value: 'Phone 2' } })
    await screen.findByRole('option', { name: /gw.example:1/ })
    fireEvent.click(screen.getByRole('button', { name: t.mobile.create }))
    await waitFor(() => expect(api.CreateMobileDevice).toHaveBeenCalled())
    const input = (api.CreateMobileDevice.mock.calls[0] as unknown[])[0] as { name: string; ramMb: number; proxyId: string }
    expect(input).toMatchObject({ name: 'Phone 2', ramMb: 3072, proxyId: 'auto' })
  })
})
