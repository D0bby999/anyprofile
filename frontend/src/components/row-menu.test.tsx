// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { service, site, store } from '../api'
import { RowMenu } from './row-menu'

const totpCode = vi.fn().mockResolvedValue({ code: '123456', secondsLeft: 12 })
const copyAccountPassword = vi.fn().mockResolvedValue({})
vi.mock('../api', async (orig) => ({
  ...(await orig<typeof import('../api')>()),
  copyText: () => Promise.resolve(),
  Api: { TOTPCode: (...a: unknown[]) => totpCode(...a), CopyAccountPassword: (...a: unknown[]) => copyAccountPassword(...a) },
}))

const row = (over: Partial<service.ProfileRow>) => service.ProfileRow.createFrom({
  id: 'a', name: 'A', sites: ['tiktok'], accounts: [], serial: 1, personaId: 'macos-m4pro-1728x1117', state: 'stopped', stateError: '', cdpPort: 0, tags: [], createdAt: '', lastOpenAt: '', ...over,
})

const acc = (id: string, over: Partial<store.Account> = {}) => store.Account.createFrom({ site: id, login: '', hasPassword: false, hasTotp: false, ...over })

const props = (over: Partial<service.ProfileRow> = {}) => ({
  row: row(over), onEdit: vi.fn(), onDuplicate: vi.fn(), onTrash: vi.fn(), onMessage: vi.fn(), onError: vi.fn(),
})

afterEach(() => { cleanup(); vi.clearAllMocks() })

describe('RowMenu', () => {
  it('hides the three copy items when nothing is stored', () => {
    render(<RowMenu sites={[]} {...props()} />)
    fireEvent.click(screen.getByRole('button', { name: 'More' }))
    expect(screen.queryByText('Copy login')).toBeNull()
    expect(screen.queryByText('Copy password')).toBeNull()
    expect(screen.queryByText('Copy 2FA code')).toBeNull()
  })

  it('copies the 2FA code when set', () => {
    render(<RowMenu sites={[]} {...props({ accounts: [acc('tiktok', { hasTotp: true })] })} />)
    fireEvent.click(screen.getByRole('button', { name: 'More' }))
    fireEvent.click(screen.getByText('Copy 2FA code'))
    expect(totpCode).toHaveBeenCalledWith('a', 'tiktok')
  })

  it('closes on Escape', () => {
    render(<RowMenu sites={[]} {...props({ accounts: [acc('tiktok', { login: 'me@x.com' })] })} />)
    fireEvent.click(screen.getByRole('button', { name: 'More' }))
    expect(screen.getByText('Copy login')).toBeTruthy()
    fireEvent.keyDown(screen.getByText('Copy login'), { key: 'Escape' })
    expect(screen.queryByText('Copy login')).toBeNull()
  })

  it('names each platform when the profile has several accounts', () => {
    const sites = [
      site.Site.createFrom({ id: 'tiktok', name: 'TikTok', group: 'video', domains: [], startUrl: '', icon: '', color: '#000', custom: false }),
      site.Site.createFrom({ id: 'gmail', name: 'Gmail', group: 'google', domains: [], startUrl: '', icon: '', color: '#EA4335', custom: false }),
    ]
    render(<RowMenu sites={sites} {...props({ sites: ['tiktok', 'gmail'], accounts: [acc('tiktok', { login: '@me' }), acc('gmail', { hasPassword: true })] })} />)
    fireEvent.click(screen.getByRole('button', { name: 'More' }))
    expect(screen.getByText('Copy login · TikTok')).toBeTruthy()
    fireEvent.click(screen.getByText('Copy password · Gmail'))
    expect(copyAccountPassword).toHaveBeenCalledWith('a', 'gmail')
  })
})
