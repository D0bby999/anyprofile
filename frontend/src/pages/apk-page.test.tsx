// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { t } from '../i18n'
import { ConfirmProvider } from '../components/confirm'
import { ToastProvider } from '../components/toast'
import { ApkPage } from './apk-page'

const mockApi = vi.hoisted(() => ({
  ListAPKs: vi.fn(),
  ListProxies: vi.fn().mockResolvedValue([]),
  PickAPKFile: vi.fn(),
  InspectAPK: vi.fn(),
  AddAPK: vi.fn(),
  UpdateAPK: vi.fn(),
  LaunchAPK: vi.fn(),
  SetAPKOrientation: vi.fn(),
  SetAPKDevice: vi.fn(),
  CreateAPKShortcut: vi.fn(),
  DeleteAPK: vi.fn(),
}))

vi.mock('../api', () => ({
  Api: mockApi,
  onDataChanged: () => () => {},
  service: {
    APKInput: { createFrom: (x: unknown) => x },
    APKUpdate: { createFrom: (x: unknown) => x },
  },
  store: {},
}))

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

const baseMobile = (pro: boolean, ready = true) => ({
  rows: [{ id: 'm1', name: 'TikTok — NL', state: 'stopped' }],
  avail: { supported: true, driver: 'avo', pro, ready, reason: '', message: '' },
  error: '',
  setError: vi.fn(),
  reload: vi.fn(),
  phases: {},
  fastStart: true,
  setFastStart: vi.fn(),
}) as never

describe('ApkPage', () => {
  it('shows the Pro upgrade card and disables Add APK when Pro is inactive', async () => {
    mockApi.ListAPKs.mockResolvedValue([])
    const onOpenPro = vi.fn()
    render(
      <ToastProvider>
        <ConfirmProvider>
          <ApkPage mobile={baseMobile(false)} onOpenPro={onOpenPro} onOpenMobile={vi.fn()} />
        </ConfirmProvider>
      </ToastProvider>,
    )
    expect(await screen.findByText(t.apk.proTitle)).toBeTruthy()
    const addBtn = screen.getByRole('button', { name: t.apk.addApk }) as HTMLButtonElement
    expect(addBtn.disabled).toBe(true)
    fireEvent.click(screen.getByRole('button', { name: t.apk.getPro }))
    expect(onOpenPro).toHaveBeenCalledTimes(1)
  })

  it('lists APK apps, filters by orientation, launches an app, updates name/proxy settings, and blocks incompatible ABI in Add dialog', async () => {
    mockApi.ListProxies.mockResolvedValue([
      { id: 'px1', label: 'US Exit', country: 'US', exitIp: '1.2.3.4', scheme: 'socks5', host: '1.2.3.4', port: 1080, usedBy: [], assignable: true, rotating: false },
    ])
    mockApi.ListAPKs.mockResolvedValue([
      {
        package: 'com.zhiliaoapp.musically',
        label: 'TikTok',
        versionName: '37.4.3',
        versionCode: 1,
        launchActivity: '',
        orientation: 'portrait',
        abis: ['arm64-v8a'],
        filePath: '',
        iconData: '',
        deviceId: 'm1',
        proxyId: 'px1',
        notes: 'Shop US',
        shortcutPath: '/Users/demo/Applications/AnyProfile Apps/TikTok.app',
        installedAt: '',
        lastOpenAt: '',
      },
      {
        package: 'com.supercell.clashofclans',
        label: 'Clash of Clans',
        versionName: '16.5',
        versionCode: 1,
        launchActivity: '',
        orientation: 'landscape',
        abis: ['arm64-v8a'],
        filePath: '',
        iconData: '',
        deviceId: '',
        proxyId: '',
        notes: '',
        identityJson: JSON.stringify({ template: 'pixel8', model: 'Pixel 8', manufacturer: 'Google', imei: '353918110000001', serial: 'AABBCCDD112233', glVendor: 'ARM', glRenderer: 'Mali-G715' }),
        androidId: 'a1b2c3d4e5f60718',
        shortcutPath: '',
        installedAt: '',
        lastOpenAt: '',
      },
    ])
    mockApi.UpdateAPK.mockResolvedValue({
      package: 'com.supercell.clashofclans',
      label: 'Clash US',
      orientation: 'landscape',
      proxyId: 'px1',
      notes: 'Main',
    })
    mockApi.LaunchAPK.mockResolvedValue(undefined)
    mockApi.PickAPKFile.mockResolvedValue('/tmp/old-x86.apk')
    mockApi.InspectAPK.mockResolvedValue({
      filePath: '/tmp/old-x86.apk',
      sizeBytes: 20 * 1024 * 1024,
      package: 'com.old.x86',
      label: 'OldX86',
      versionName: '1.0',
      versionCode: 1,
      minSdk: 21,
      targetSdk: 33,
      launchActivity: '',
      orientation: 'auto',
      abis: ['x86_64'],
      isSplitBundle: false,
      iconData: '',
      abiCompatible: false,
      abiWarning: 'incompatible ABI: package only includes x86_64',
    })

    render(
      <ToastProvider>
        <ConfirmProvider>
          <ApkPage mobile={baseMobile(true)} onOpenPro={vi.fn()} onOpenMobile={vi.fn()} />
        </ConfirmProvider>
      </ToastProvider>,
    )

    expect(await screen.findByText('TikTok')).toBeTruthy()
    expect(screen.getByText('Clash of Clans')).toBeTruthy()

    // Orientation filter
    fireEvent.click(screen.getByRole('button', { name: t.apk.landscapeTab }))
    expect(screen.queryByText('TikTok')).toBeNull()
    expect(screen.getByText('Clash of Clans')).toBeTruthy()

    // Launch app
    fireEvent.click(screen.getByRole('button', { name: t.apk.open }))
    await waitFor(() => {
      expect(mockApi.LaunchAPK).toHaveBeenCalledWith('com.supercell.clashofclans', '')
    })

    // Open Web-style split drawer and apply proxy change
    fireEvent.click(screen.getByRole('button', { name: `${t.apk.settings} Clash of Clans` }))
    fireEvent.change(screen.getByLabelText(t.create.proxy), { target: { value: 'px1' } })
    fireEvent.click(screen.getByRole('button', { name: t.drawer.apply }))
    await waitFor(() => {
      expect(mockApi.UpdateAPK).toHaveBeenCalledWith(expect.objectContaining({
        package: 'com.supercell.clashofclans',
        proxyId: 'px1',
      }))
    })

    // Rotate device fingerprint (IMEI, Serial & Android ID)
    fireEvent.click(screen.getByRole('button', { name: t.mobile.regenIdentity }))
    await waitFor(() => {
      expect(mockApi.UpdateAPK).toHaveBeenCalledWith(expect.objectContaining({
        package: 'com.supercell.clashofclans',
        identityTemplate: 'regen:pixel8',
      }))
    })

    // Add APK with incompatible ABI disables confirm button
    fireEvent.click(screen.getByRole('button', { name: t.apk.addApk }))
    expect(await screen.findByText(/incompatible ABI/i)).toBeTruthy()
    const installBtn = screen.getByRole('button', { name: t.apk.installBtn }) as HTMLButtonElement
    expect(installBtn.disabled).toBe(true)
  })
})
