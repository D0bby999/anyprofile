// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { t } from '../i18n'
import { service, site } from '../api'

let result: unknown = { imported: 182, skipped: 4, domains: ['tiktok.com', 'google.com', 'a.com', 'b.com'], siteMismatch: '' }
vi.mock('../api', async (orig) => ({
  ...(await orig<typeof import('../api')>()),
  Api: { ImportCookies: () => Promise.resolve(result), ExportCookies: () => Promise.resolve(3) },
}))
const { RowMenu } = await import('./row-menu')
const { ToastProvider } = await import('./toast')

const sites = [site.Site.createFrom({ id: 'tiktok', name: 'TikTok', group: 'video', domains: ['tiktok.com'], startUrl: '', icon: '', color: '#000', custom: false })]
const row = (state: string) => service.ProfileRow.createFrom({ id: 'a', name: 'A', sites: ['tiktok'], state, login: '', hasPassword: false, hasTotp: false, pinned: false })
const menu = (state = 'stopped') => {
  render(<ToastProvider><RowMenu row={row(state)} sites={sites} onEdit={vi.fn()} onDuplicate={vi.fn()} onTrash={vi.fn()} onMessage={vi.fn()} onError={vi.fn()} /></ToastProvider>)
  fireEvent.click(screen.getByRole('button', { name: t.menu.more }))
}

afterEach(cleanup)

describe('cookie actions', () => {
  it('locks Export while the profile is closed', () => {
    menu('stopped')
    expect((screen.getByRole('menuitem', { name: t.cookies.export }) as HTMLButtonElement).disabled).toBe(true)
  })

  it('reports how many cookies came in', async () => {
    menu()
    fireEvent.click(screen.getByRole('menuitem', { name: t.cookies.import }))
    expect(await screen.findByText(t.cookies.imported(182, 'tiktok.com, google.com', 2, 4))).toBeTruthy()
  })

  it('notes when the cookies belong to another platform', async () => {
    result = { imported: 10, skipped: 0, domains: ['youtube.com'], siteMismatch: 'youtube.com' }
    menu()
    fireEvent.click(screen.getByRole('menuitem', { name: t.cookies.import }))
    expect(await screen.findByText(t.cookies.mismatch('youtube.com', 'TikTok'))).toBeTruthy()
  })
})
