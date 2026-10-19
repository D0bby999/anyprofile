// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { t } from '../i18n'

const none = { active: false, key: '', state: '', paidThrough: '', version: '', notes: '', checkedAt: '', error: '', waiting: false }
const active = { ...none, active: true, key: 'APRO-•••••-•••••-•••••-DDDDD', state: 'active', paidThrough: '2027-10-19T00:00:00Z', version: '1.2.0', notes: 'TikTok upload.', checkedAt: '2026-10-19T12:00:00Z' }
let status: Record<string, unknown> = none
const api = {
  ProStatus: vi.fn(() => Promise.resolve(status)),
  ProBuy: vi.fn(() => Promise.resolve()),
  ProCancelBuy: vi.fn(() => Promise.resolve()),
  ProActivate: vi.fn(() => Promise.resolve(active)),
  ProCheck: vi.fn(() => Promise.resolve({ ...active, version: '1.3.0' })),
  ProDeactivate: vi.fn(() => Promise.resolve()),
  ProOpenPortal: vi.fn(() => Promise.resolve()),
  ProOpenScripts: vi.fn(() => Promise.resolve()),
}
vi.mock('../api', async (orig) => ({
  ...(await orig<typeof import('../api')>()),
  onDataChanged: () => () => {},
  onProEvent: () => () => {},
  Api: api,
}))
const { ProSettings } = await import('./pro-settings')
const { ConfirmProvider } = await import('./confirm')

const show = () => render(<ConfirmProvider><ProSettings /></ConfirmProvider>)

describe('ProSettings', () => {
  beforeEach(() => { vi.clearAllMocks(); status = none })
  afterEach(cleanup)

  it('offers both plans and buys', async () => {
    show()
    const year = await screen.findByRole('button', { name: t.pro.buyYear })
    fireEvent.click(year)
    await waitFor(() => expect(api.ProBuy).toHaveBeenCalledWith('year'))
    fireEvent.click(screen.getByRole('button', { name: t.pro.buyMonth }))
    await waitFor(() => expect(api.ProBuy).toHaveBeenCalledWith('month'))
  })

  it('shows the wait for a payment, which can be stopped', async () => {
    status = { ...none, waiting: true }
    show()
    fireEvent.click(await screen.findByRole('button', { name: t.pro.stopWaiting }))
    await waitFor(() => expect(api.ProCancelBuy).toHaveBeenCalled())
    expect(screen.queryByRole('button', { name: t.pro.buyYear })).toBeNull()
  })

  it('activates a pasted key', async () => {
    show()
    const input = await screen.findByRole('textbox', { name: t.pro.keyLabel })
    expect(screen.getByRole('button', { name: t.pro.activate })).toHaveProperty('disabled', true)
    fireEvent.change(input, { target: { value: 'APRO-AAAAA-BBBBB-CCCCC-DDDDD' } })
    fireEvent.click(screen.getByRole('button', { name: t.pro.activate }))
    await waitFor(() => expect(api.ProActivate).toHaveBeenCalledWith('APRO-AAAAA-BBBBB-CCCCC-DDDDD'))
  })

  it('shows an active key, its pack, and checks for a newer one', async () => {
    status = active
    show()
    expect(await screen.findByText(t.pro.pack('1.2.0'))).toBeTruthy()
    expect(screen.getByText('TikTok upload.')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: t.pro.check }))
    await waitFor(() => expect(api.ProCheck).toHaveBeenCalled())
  })

  it('removes the key only after confirming', async () => {
    status = active
    show()
    fireEvent.click(await screen.findByRole('button', { name: t.pro.remove }))
    fireEvent.click(await screen.findByRole('button', { name: t.pro.confirmRemove.confirm }))
    await waitFor(() => expect(api.ProDeactivate).toHaveBeenCalled())
  })

  it('names a server error in words', async () => {
    api.ProActivate.mockRejectedValueOnce(new Error('anyprofile:pro_too_many_devices: in use on 3 computers'))
    show()
    fireEvent.change(await screen.findByRole('textbox', { name: t.pro.keyLabel }), { target: { value: 'APRO-X' } })
    fireEvent.click(screen.getByRole('button', { name: t.pro.activate }))
    expect(await screen.findByText(t.errors.pro_too_many_devices)).toBeTruthy()
  })
})
