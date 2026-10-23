// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { t } from '../i18n'
import { ConfirmProvider } from '../components/confirm'
import { ToastProvider } from '../components/toast'

const api = {
  ListProxies: vi.fn(() => Promise.resolve([{ id: 'px', label: 'gw.example:1', country: 'JP', rotating: false, usedBy: [] }])),
  StartMobileDevice: vi.fn(() => Promise.resolve({ warning: '' })),
  BackupMobileDevice: vi.fn(() => Promise.resolve()),
  ShowMobileDevice: vi.fn(() => Promise.resolve()),
  InstallMobileRuntime: vi.fn(() => Promise.resolve()),
  CreateMobileDevice: vi.fn(() => Promise.resolve({ id: 'n' })),
}
vi.mock('../api', async (orig) => ({
  ...(await orig<typeof import('../api')>()),
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
    expect(screen.getByRole('status').textContent).toContain('Install LDPlayer 9.')
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
