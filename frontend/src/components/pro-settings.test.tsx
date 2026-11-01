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
  ProBuyGift: vi.fn(() => Promise.resolve()),
  ProCancelBuy: vi.fn(() => Promise.resolve()),
  ProActivate: vi.fn(() => Promise.resolve(active)),
  ProRedeemGift: vi.fn(() => Promise.resolve(active)),
  ProCheck: vi.fn(() => Promise.resolve({ ...active, version: '1.3.0' })),
  ProDeactivate: vi.fn(() => Promise.resolve()),
  ProDevices: vi.fn(() => Promise.resolve([{ install_id: 'device-1', name: 'Mac', platform: 'darwin-arm64', last_seen: 1, current: false }])),
  ProRemoveDevice: vi.fn(() => Promise.resolve()),
  ProOpenPortal: vi.fn(() => Promise.resolve()),
  ProOpenScripts: vi.fn(() => Promise.resolve()),
  ProAccountStatus: vi.fn(() => Promise.resolve({ signedIn: false, email: '', name: '' })),
  ProAccountLogin: vi.fn(() => Promise.resolve({ signedIn: false, email: '', name: '' })),
  ProAccountSignOut: vi.fn(() => Promise.resolve()),
  ProAccountLink: vi.fn(() => Promise.resolve({ signedIn: true, email: 'a@example.com', name: '' })),
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
const openAdvanced = async () => fireEvent.click(await screen.findByRole('button', { name: t.pro.haveKey }))

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

  it('opens the paid-gift checkout without starting a key wait', async () => {
    show()
    fireEvent.click(await screen.findByRole('button', { name: t.pro.buyGift }))
    await waitFor(() => expect(api.ProBuyGift).toHaveBeenCalled())
    expect(api.ProBuy).not.toHaveBeenCalled()
  })

  it('shows the wait for a payment, which can be stopped', async () => {
    status = { ...none, waiting: true }
    show()
    fireEvent.click(await screen.findByRole('button', { name: t.pro.stopWaiting }))
    await waitFor(() => expect(api.ProCancelBuy).toHaveBeenCalled())
    expect(screen.queryByRole('button', { name: t.pro.buyYear })).toBeNull()
  })

  it('redeems a gift code without Stripe', async () => {
    show()
    await openAdvanced()
    const input = await screen.findByRole('textbox', { name: t.pro.giftLabel })
    fireEvent.change(input, { target: { value: 'APRO-GIFT-AAAA-BBBB-CCCC-DDDD' } })
    fireEvent.click(screen.getByRole('button', { name: t.pro.redeem }))
    await waitFor(() => expect(api.ProRedeemGift).toHaveBeenCalledWith('APRO-GIFT-AAAA-BBBB-CCCC-DDDD'))
  })

  it('activates a pasted key', async () => {
    show()
    await openAdvanced()
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

  it('lists devices and frees another machine', async () => {
    status = active
    show()
    const removeButtons = await screen.findAllByRole('button', { name: t.pro.removeDevice })
    fireEvent.click(removeButtons[0])
    await waitFor(() => expect(api.ProRemoveDevice).toHaveBeenCalledWith('device-1'))
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
    await openAdvanced()
    fireEvent.change(await screen.findByRole('textbox', { name: t.pro.keyLabel }), { target: { value: 'APRO-X' } })
    fireEvent.click(screen.getByRole('button', { name: t.pro.activate }))
    expect(await screen.findByText(t.errors.pro_too_many_devices)).toBeTruthy()
  })
})
