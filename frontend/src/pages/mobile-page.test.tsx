// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { t } from '../i18n'
import { ConfirmProvider } from '../components/confirm'
import { ToastProvider } from '../components/toast'

const api = {
  ListProxies: vi.fn(() => Promise.resolve([{ id: 'px', label: 'gw.example:1', country: 'JP', exitIp: '2.2.2.2', rotating: false, usedBy: [], assignable: true }])),
  StartMobileDevice: vi.fn(() => Promise.resolve({ warning: '' })),
  BackupMobileDevice: vi.fn(() => Promise.resolve()),
  InstallMobileApps: vi.fn(() => Promise.resolve(2)),
  InstallMobileAurora: vi.fn(() => Promise.resolve()),
  ShowMobileDevice: vi.fn(() => Promise.resolve()),
  InstallMobileRuntime: vi.fn(() => Promise.resolve()),
  CreateMobileDevice: vi.fn(() => Promise.resolve({ id: 'n' })),
  TrashMobileDevice: vi.fn(() => Promise.resolve()),
  RestoreMobileDevice: vi.fn(() => Promise.resolve()),
  OpenMobileRuntimeSource: vi.fn(() => Promise.resolve()),
  MobileFastStart: vi.fn(() => Promise.resolve(true)),
  SetMobileFastStart: vi.fn(() => Promise.resolve()),
  ScreenshotMobileDevice: vi.fn(() => Promise.resolve('/tmp/shot.jpg')),
  MobilePreviewFrame: vi.fn(() => Promise.resolve('abc')),
  TapMobileDevice: vi.fn(() => Promise.resolve()),
  PressMobileKey: vi.fn(() => Promise.resolve()),
  StartMobileRecording: vi.fn(() => Promise.resolve()),
  StopMobileRecording: vi.fn(() => Promise.resolve('/tmp/clip.mp4')),
  MobileWindowOnTop: vi.fn(() => Promise.resolve(false)),
  SetMobileWindowOnTop: vi.fn(() => Promise.resolve()),
  UpdateMobileDevice: vi.fn(() => Promise.resolve()),
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
const openRowMenu = async (index = 0) => fireEvent.click(screen.getAllByRole('button', { name: t.menu.more })[index])
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
    await openRowMenu()
    fireEvent.click(screen.getByRole('menuitem', { name: t.mobile.backup }))
    await waitFor(() => expect(api.BackupMobileDevice).toHaveBeenCalledWith('d1'))
  })

  it('toggles fast start and saves the setting', async () => {
    page([device()], avail())
    const box = screen.getByRole('checkbox', { name: t.mobile.fastStart }) as HTMLInputElement
    expect(box.checked).toBe(true)
    fireEvent.click(box)
    await waitFor(() => expect(api.SetMobileFastStart).toHaveBeenCalledWith(false))
  })

  it('opens the in-app viewer', async () => {
    page([device({ state: 'running' })], avail())
    await openRowMenu()
    fireEvent.click(screen.getByRole('menuitem', { name: t.mobile.viewInApp }))
    await waitFor(() => expect(api.MobilePreviewFrame).toHaveBeenCalledWith('d1'))
    expect(await screen.findByRole('dialog')).toBeTruthy()
  })

  it('takes a screenshot from the row menu', async () => {
    page([device({ state: 'running' })], avail())
    await openRowMenu()
    fireEvent.click(screen.getByRole('menuitem', { name: t.mobile.screenshot }))
    await waitFor(() => expect(api.ScreenshotMobileDevice).toHaveBeenCalledWith('d1'))
  })

  it('downloads Android when the runtime is missing', async () => {
    page([], avail({ ready: false, reason: 'anymobile_runtime_missing', message: '' }))
    fireEvent.click(screen.getByRole('button', { name: t.mobile.install }))
    await waitFor(() => expect(api.InstallMobileRuntime).toHaveBeenCalled())
  })

  it('offers the official LDPlayer download and checks again when missing', () => {
    const reload = vi.fn()
    const mobile = { rows: [], avail: avail({ driver: 'ldplayer', ready: false, reason: 'ldplayer_missing' }), error: '', setError: vi.fn(), reload } as never
    render(<ConfirmProvider><ToastProvider><MobilePage mobile={mobile} onOpenPro={vi.fn()} onOpenSettings={vi.fn()} /></ToastProvider></ConfirmProvider>)
    expect(screen.getByText(t.mobile.ldplayerLite)).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: t.mobile.ldplayerDownload }))
    expect(openExternal).toHaveBeenCalledWith('https://www.ldplayer.net/')
    fireEvent.click(screen.getByRole('button', { name: t.mobile.ldplayerCheck }))
    expect(reload).toHaveBeenCalled()
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
    await openRowMenu()
    fireEvent.click(screen.getByRole('menuitem', { name: t.mobile.trash }))
    await waitFor(() => expect(api.TrashMobileDevice).toHaveBeenCalledWith('d1'))
    fireEvent.click(await screen.findByRole('button', { name: t.common.undo }))
    await waitFor(() => expect(api.RestoreMobileDevice).toHaveBeenCalledWith('d1'))
  })

  it('says Beta, and Coming soon instead of a Download that can only fail while the runtime is not out', () => {
    const missing = { ready: false, reason: 'anymobile_runtime_missing' }
    const comp = (latest: string) => ({ runtime: { engine: 'anymobile', installed: '', latest }, image: { engine: 'anymobile-image', installed: '', latest: '' } })
    page([], avail({ ...missing, install: comp('') }))
    expect(screen.getByText(t.mobile.beta)).toBeTruthy()
    expect(screen.getByText(t.mobile.comingSoon)).toBeTruthy()
    expect(screen.queryByRole('button', { name: t.mobile.install })).toBeNull()
    cleanup()
    page([], avail({ ...missing, install: comp('1.0.0') }))
    expect(screen.getByText(t.mobile.installText)).toBeTruthy()
    expect(screen.getByRole('button', { name: t.mobile.install })).toBeTruthy()
  })

  it('does not publish the runtime source link', () => {
    const comp = { runtime: { engine: 'anymobile', installed: '1.0.0', latest: '1.0.0' }, image: { engine: 'anymobile-image', installed: '23.2', latest: '' } }
    page([device()], avail({ install: comp }))
    expect(screen.queryByRole('link', { name: t.mobile.runtimeSource })).toBeNull()
  })

  it('links to the guide for installing apps', () => {
    page([device()], avail())
    fireEvent.click(screen.getByRole('link', { name: 'Aurora Store' }))
    expect(openExternal).toHaveBeenCalledWith('https://f-droid.org/packages/com.aurora.store/')
    fireEvent.click(screen.getByRole('link', { name: t.mobile.apkGuide }))
    expect(openExternal).toHaveBeenCalledWith('https://github.com/D0bby999/anyprofile/blob/master/docs/mobile-apps.md')
  })

  it('installs apps on a running device only', async () => {
    page([device({ state: 'running' })], avail())
    fireEvent.click(screen.getByRole('button', { name: t.mobile.installApps }))
    await waitFor(() => expect(api.InstallMobileApps).toHaveBeenCalledWith('d1'))
    expect(await screen.findByText(t.mobile.appsInstalled(2))).toBeTruthy()
    await openRowMenu()
    fireEvent.click(screen.getByRole('menuitem', { name: t.mobile.installAurora }))
    await waitFor(() => expect(api.InstallMobileAurora).toHaveBeenCalledWith('d1'))
    expect(await screen.findByText(t.mobile.auroraInstalled)).toBeTruthy()
    cleanup()
    page([device()], avail())
    expect(screen.queryByRole('button', { name: t.mobile.installApps })).toBeNull()
  })

  it('shows the current startup phase', () => {
    const mobile = { rows: [device({ state: 'starting' })], avail: avail(), error: '', setError: vi.fn(), reload: vi.fn(), phases: { d1: 'boot' } } as never
    render(<ConfirmProvider><ToastProvider><MobilePage mobile={mobile} onOpenPro={vi.fn()} onOpenSettings={vi.fn()} /></ToastProvider></ConfirmProvider>)
    expect(screen.getByRole('status').textContent).toBe(t.mobile.startPhase.boot)
  })

  it('shows Close and Show window for a running device and locks backup', async () => {
    page([device({ state: 'running' })], avail())
    expect(screen.getByRole('button', { name: t.mobile.close })).toBeTruthy()
    await openRowMenu()
    expect((screen.getByRole('menuitem', { name: t.mobile.backup }) as HTMLButtonElement).disabled).toBe(true)
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

  it('opens the rightbar drawer when clicking a row or selecting Edit in row menu', async () => {
    page([device({ id: 'd1', name: 'Phone 1' }), device({ id: 'd2', name: 'Phone 2' })], avail())
    expect(screen.queryByRole('complementary')).toBeNull()

    // Clicking row opens drawer
    fireEvent.click(screen.getByText('Phone 1'))
    const drawer = await screen.findByRole('complementary', { name: t.mobile.drawerLabel('Phone 1') })
    expect(drawer).toBeTruthy()
    expect(screen.queryByRole('dialog')).toBeNull() // Not a modal!

    // Close drawer
    fireEvent.click(screen.getByRole('button', { name: t.drawer.close }))
    expect(screen.queryByRole('complementary')).toBeNull()

    // Row menu Edit opens drawer
    await openRowMenu()
    fireEvent.click(screen.getByRole('menuitem', { name: t.mobile.edit }))
    expect(await screen.findByRole('complementary', { name: t.mobile.drawerLabel('Phone 1') })).toBeTruthy()
  })

  it('filters mobile devices via search field', () => {
    page([device({ id: 'd1', name: 'TikTok US', serial: 101 }), device({ id: 'd2', name: 'Shopee VN', serial: 202 })], avail())
    expect(screen.getByText('TikTok US')).toBeTruthy()
    expect(screen.getByText('Shopee VN')).toBeTruthy()

    const searchInput = screen.getByLabelText(t.mobile.search)
    fireEvent.change(searchInput, { target: { value: 'tiktok' } })
    expect(screen.getByText('TikTok US')).toBeTruthy()
    expect(screen.queryByText('Shopee VN')).toBeNull()

    fireEvent.change(searchInput, { target: { value: '#202' } })
    expect(screen.queryByText('TikTok US')).toBeNull()
    expect(screen.getByText('Shopee VN')).toBeTruthy()
  })

  it('switches tabs in the mobile rightbar drawer and updates device configuration', async () => {
    page([device({ id: 'd1', name: 'Phone 1', ramMb: 3072, cpus: 4, preset: 'balanced' })], avail())
    fireEvent.click(screen.getByText('Phone 1'))
    const drawer = await screen.findByRole('complementary')

    // Switch to Hardware tab
    fireEvent.click(screen.getByRole('tab', { name: t.mobile.hardware }))
    expect(screen.getByText(t.mobile.presetAutomation + ' (2 CPU · 2 GB)')).toBeTruthy()

    // Switch to Activity tab
    fireEvent.click(screen.getByRole('tab', { name: t.mobile.activity }))
    expect(within(drawer).getAllByText('#1').length).toBeGreaterThanOrEqual(1)
    expect(within(drawer).getByRole('button', { name: t.mobile.backupNow })).toBeTruthy()
  })

  it('renders order numbering and drag grip for each mobile device', () => {
    page([
      device({ id: 'd1', name: 'Phone 1', serial: 1 }),
      device({ id: 'd2', name: 'Phone 2', serial: 2 }),
    ], avail())

    expect(screen.getByRole('columnheader', { name: t.profiles.colNo })).toBeTruthy()
    const grips = screen.getAllByLabelText(/Move Phone/)
    expect(grips.length).toBe(2)
    expect(screen.getByText('1')).toBeTruthy()
    expect(screen.getByText('2')).toBeTruthy()
  })

  it('supports selecting individual rows and bulk deletion', async () => {
    page([
      device({ id: 'd1', name: 'Phone 1' }),
      device({ id: 'd2', name: 'Phone 2' }),
    ], avail())

    const box1 = screen.getByRole('checkbox', { name: 'Phone 1' })
    fireEvent.click(box1)

    // Selection bar appears
    expect(screen.getByText(t.profiles.selected(1))).toBeTruthy()
    const delBtn = screen.getByRole('button', { name: t.common.delete })
    fireEvent.click(delBtn)

    await waitFor(() => expect(api.TrashMobileDevice).toHaveBeenCalledWith('d1'))
  })

  it('toggles all devices via the header checkbox', () => {
    page([
      device({ id: 'd1', name: 'Phone 1' }),
      device({ id: 'd2', name: 'Phone 2' }),
    ], avail())

    const selectAll = screen.getByRole('checkbox', { name: t.profiles.selectAll(2) }) as HTMLInputElement
    expect(selectAll.checked).toBe(false)

    fireEvent.click(selectAll)
    expect(screen.getByText(t.profiles.selected(2))).toBeTruthy()

    // Clicking cancel clears selection
    fireEvent.click(screen.getByRole('button', { name: t.common.cancel }))
    expect(screen.queryByText(t.profiles.selected(2))).toBeNull()
  })
})
