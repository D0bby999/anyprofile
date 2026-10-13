// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { service } from '../api'
import { AccountPanel } from './account-panel'

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
  },
}))

const row = (over: Partial<service.ProfileRow>) => service.ProfileRow.createFrom({
  id: 'a', name: 'A', site: 'other', serial: 1, personaId: 'macos-m4pro-1728x1117', login: '',
  hasPassword: false, hasTotp: false, state: 'stopped', stateError: '', cdpPort: 0, tags: [], createdAt: '', lastOpenAt: '', ...over,
})

afterEach(() => { cleanup(); vi.clearAllMocks() })

describe('AccountPanel', () => {
  it('shows a spaced code when 2FA is set', async () => {
    render(<AccountPanel row={row({ hasTotp: true })} login="" onLogin={vi.fn()} onError={vi.fn()} />)
    await waitFor(() => expect(screen.getByText('287 082')).toBeTruthy())
  })

  it('saves a password and clears the field', async () => {
    render(<AccountPanel row={row({})} login="" onLogin={vi.fn()} onError={vi.fn()} />)
    const pwInput = document.querySelector('input[type="password"]') as HTMLInputElement
    fireEvent.change(pwInput, { target: { value: 'pw1' } })
    fireEvent.click(screen.getByRole('button', { name: /Save password/ }))
    await waitFor(() => expect(setAccountPassword).toHaveBeenCalledWith('a', 'pw1'))
    expect(pwInput.value).toBe('')
  })

  it('copies a stored password through Go', async () => {
    render(<AccountPanel row={row({ hasPassword: true })} login="" onLogin={vi.fn()} onError={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Copy' }))
    await waitFor(() => expect(copyAccountPassword).toHaveBeenCalledWith('a'))
  })
})
