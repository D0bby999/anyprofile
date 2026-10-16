// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { t } from '../i18n'

const status = { current: '0.1.0', latest: '0.2.0', date: '2026-10-20', notes: 'Faster start.', available: true, checkedAt: '', auto: true }
const api = {
  UpdateStatus: vi.fn(() => Promise.resolve(status)),
  CheckUpdate: vi.fn(() => Promise.resolve({ ...status, latest: '0.1.0', available: false, notes: '' })),
  InstallUpdate: vi.fn(() => Promise.resolve()),
  SetUpdateAuto: vi.fn(() => Promise.resolve()),
}
vi.mock('../api', async (orig) => ({
  ...(await orig<typeof import('../api')>()),
  onDataChanged: () => () => {},
  onUpdateProgress: () => () => {},
  Api: api,
}))
const { UpdateSettings } = await import('./update-settings')
const { ConfirmProvider } = await import('./confirm')

const show = (open = 0) => render(<ConfirmProvider><UpdateSettings openProfiles={open} /></ConfirmProvider>)

describe('UpdateSettings', () => {
  beforeEach(() => vi.clearAllMocks())
  afterEach(cleanup)

  it('offers the new version with its notes and installs after confirming', async () => {
    show()
    expect(await screen.findByText(t.update.available('0.2.0'))).toBeTruthy()
    expect(screen.getByText('Faster start.')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: t.update.install }))
    await screen.findByRole('dialog')
    const buttons = screen.getAllByRole('button', { name: t.update.confirm('0.2.0').confirm })
    fireEvent.click(buttons[buttons.length - 1]) // the dialog's, on top
    await waitFor(() => expect(api.InstallUpdate).toHaveBeenCalledOnce())
  })

  it('waits for open profiles to close', async () => {
    show(2)
    const btn = await screen.findByRole('button', { name: t.update.install })
    expect((btn as HTMLButtonElement).disabled).toBe(true)
    expect(screen.getByText(t.update.closeFirst(2))).toBeTruthy()
  })

  it('checks on demand and turns automatic checks off', async () => {
    show()
    fireEvent.click(await screen.findByRole('button', { name: t.update.check }))
    expect(await screen.findByText(t.update.upToDate)).toBeTruthy()
    expect(screen.queryByRole('button', { name: t.update.install })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: t.update.autoOff }))
    await waitFor(() => expect(api.SetUpdateAuto).toHaveBeenCalledWith(false))
  })
})
