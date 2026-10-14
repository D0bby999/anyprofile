// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { service, site, store } from '../api'
import { t } from '../i18n'
import { AccountsPanel } from './account-panel'
import { ConfirmProvider } from './confirm'
import { ToastProvider } from './toast'

const setAccountPassword = vi.fn().mockResolvedValue({})
const copyAccountPassword = vi.fn().mockResolvedValue({})
const totpCode = vi.fn().mockResolvedValue({ code: '287082', secondsLeft: 12 })
const clearTotp = vi.fn().mockResolvedValue({})
const clearAccountPassword = vi.fn().mockResolvedValue({})
vi.mock('../api', async (orig) => ({
  ...(await orig<typeof import('../api')>()),
  copyText: () => Promise.resolve(),
  Api: {
    SetAccountPassword: (...a: unknown[]) => setAccountPassword(...a),
    CopyAccountPassword: (...a: unknown[]) => copyAccountPassword(...a),
    TOTPCode: (...a: unknown[]) => totpCode(...a),
    ClearTOTP: (...a: unknown[]) => clearTotp(...a),
    ClearAccountPassword: (...a: unknown[]) => clearAccountPassword(...a),
    SetAccountLogin: (...a: unknown[]) => setAccountLogin(...a),
    ForgetAccount: (...a: unknown[]) => forgetAccount(...a),
  },
}))

const setAccountLogin = vi.fn().mockResolvedValue({})
const forgetAccount = vi.fn().mockResolvedValue({})

const acc = (site: string, over: Partial<store.Account> = {}) => store.Account.createFrom({ site, login: '', hasPassword: false, hasTotp: false, ...over })
const row = (over: Partial<service.ProfileRow>) => service.ProfileRow.createFrom({
  id: 'a', name: 'A', sites: ['tiktok', 'gmail'], accounts: [], serial: 1, personaId: 'macos-m4pro-1728x1117',
  state: 'stopped', stateError: '', cdpPort: 0, tags: [], createdAt: '', lastOpenAt: '', ...over,
})
const sites = [
  site.Site.createFrom({ id: 'tiktok', name: 'TikTok', group: 'video', domains: [], startUrl: '', icon: 'tiktok', color: '#000000', custom: false }),
  site.Site.createFrom({ id: 'gmail', name: 'Gmail', group: 'google', domains: [], startUrl: '', icon: 'gmail', color: '#EA4335', custom: false }),
  site.Site.createFrom({ id: 'etsy', name: 'Etsy', group: 'commerce', domains: [], startUrl: '', icon: 'etsy', color: '#F16521', custom: false }),
]
const panel = (r: service.ProfileRow) => render(<ConfirmProvider><ToastProvider><AccountsPanel row={r} sites={sites} onError={vi.fn()} /></ToastProvider></ConfirmProvider>)

afterEach(() => { cleanup(); vi.clearAllMocks() })

describe('AccountsPanel', () => {
  it('shows one account per platform, with a spaced 2FA code on the one that has it', async () => {
    panel(row({ accounts: [acc('gmail', { hasTotp: true })] }))
    expect(screen.getByRole('heading', { name: 'TikTok' })).toBeTruthy()
    expect(screen.getByRole('heading', { name: 'Gmail' })).toBeTruthy()
    await waitFor(() => expect(screen.getByText('287 082')).toBeTruthy())
    expect(totpCode).toHaveBeenCalledWith('a', 'gmail')
    expect(totpCode).not.toHaveBeenCalledWith('a', 'tiktok')
  })

  it('saves a password and a login for the right platform', async () => {
    panel(row({}))
    const pwInput = document.querySelectorAll('input[type="password"]')[1] as HTMLInputElement
    fireEvent.change(pwInput, { target: { value: 'pw1' } })
    fireEvent.click(screen.getAllByRole('button', { name: /Save password/ })[1])
    await waitFor(() => expect(setAccountPassword).toHaveBeenCalledWith('a', 'gmail', 'pw1'))
    expect(pwInput.value).toBe('')
    const login = screen.getAllByLabelText(/Login/)[0]
    fireEvent.change(login, { target: { value: ' @me ' } })
    fireEvent.blur(login)
    await waitFor(() => expect(setAccountLogin).toHaveBeenCalledWith('a', 'tiktok', '@me'))
  })

  it('copies a stored password through Go', async () => {
    panel(row({ accounts: [acc('tiktok', { hasPassword: true })] }))
    fireEvent.click(screen.getByRole('button', { name: 'Copy' }))
    await waitFor(() => expect(copyAccountPassword).toHaveBeenCalledWith('a', 'tiktok'))
  })

  it('keeps the account of a removed platform until it is forgotten', async () => {
    panel(row({ sites: ['tiktok'], accounts: [acc('etsy', { login: 'shop@x.com' })] }))
    expect(screen.getByText(t.account.removedNote)).toBeTruthy()
    expect(screen.getByDisplayValue('shop@x.com')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: t.account.forget }))
    fireEvent.click(await screen.findByRole('button', { name: t.confirm.forgetAccount('Etsy').confirm }))
    await waitFor(() => expect(forgetAccount).toHaveBeenCalledWith('a', 'etsy'))
  })
})
