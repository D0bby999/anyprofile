// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { t } from '../i18n'
import type { useMobile } from '../hooks/use-mobile'
import { MobileEngineSettings } from './mobile-engine-settings'

const api = vi.hoisted(() => ({ ProCheck: vi.fn(), InstallMobileRuntime: vi.fn() }))
vi.mock('../api', () => ({ Api: api, onEngineProgress: () => () => {}, openExternal: vi.fn() }))
afterEach(() => { cleanup(); vi.resetAllMocks() })

function mobile(state = 'stopped') {
  const metal = { runtime: { engine: 'anymobile-avo', installed: 'dev', latest: '1.0.0' }, image: { installed: '16968706' }, canUpdate: true }
  return { avail: { supported: true, pro: true, install: metal, engines: [metal] }, rows: [{ state }], reload: vi.fn() } as unknown as ReturnType<typeof useMobile>
}

it('shows Metal runtime and lets Pro users check and manually update', async () => {
  const data = mobile()
  render(<MobileEngineSettings mobile={data} />)
  expect(screen.getByText('AnyMobile Metal')).toBeTruthy()
  expect(screen.getByText(t.mobile.engineSettings.installed(t.mobile.engineSettings.development))).toBeTruthy()
  fireEvent.click(screen.getByRole('button', { name: t.update.check }))
  expect(await screen.findByText(t.mobile.engineSettings.checked)).toBeTruthy()
  expect(api.ProCheck).toHaveBeenCalledOnce()
  fireEvent.click(screen.getByRole('button', { name: t.mobile.engineUpdateBtn }))
  expect(await screen.findByText(t.mobile.engineSettings.updated)).toBeTruthy()
  expect(api.InstallMobileRuntime).toHaveBeenCalledOnce()
  expect(data.reload).toHaveBeenCalledTimes(2)
})

it('requires devices to be closed and shows server failures', async () => {
  api.ProCheck.mockRejectedValue(new Error('network offline'))
  render(<MobileEngineSettings mobile={mobile('running')} />)
  expect((screen.getByRole('button', { name: t.mobile.engineUpdateBtn }) as HTMLButtonElement).disabled).toBe(true)
  expect(screen.getByText(t.mobile.engineSettings.closeFirst)).toBeTruthy()
  fireEvent.click(screen.getByRole('button', { name: t.update.check }))
  await waitFor(() => expect(screen.getByRole('alert').textContent).toContain('network offline'))
  expect(api.InstallMobileRuntime).not.toHaveBeenCalled()
})
