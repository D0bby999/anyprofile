// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { t } from '../i18n'
import { ConfirmProvider } from '../components/confirm'
import { ToastProvider } from '../components/toast'
import { partnersOf } from '../lib/partners'

const proxy = (id: string, extra = {}) => ({ id, label: `gw.example:${id}`, scheme: 'http', host: 'gw.example', port: 10001, note: '',
  country: 'US', exitIp: '', timezone: '', status: 'unchecked', checkedAt: '', expiresAt: '', rotating: false, usedBy: [], ...extra })
const openExternal = vi.fn()
vi.mock('../api', async (orig) => ({
  ...(await orig<typeof import('../api')>()),
  openExternal: (u: string) => openExternal(u),
  onDataChanged: () => () => {},
  Api: { ListProxies: () => Promise.resolve([proxy('1', { exitIp: '1.2.3.4', timezone: 'America/New_York', status: 'ok' }), proxy('2'),
    proxy('3', { exitIp: '5.6.7.8', timezone: 'America/New_York', exitIp6: '2001:db8::7', timezone6: 'America/Chicago' })]) },
}))
const { ProxiesPage } = await import('./proxies-page')

afterEach(cleanup)

describe('ProxiesPage', () => {
  it('shows the exit IP with its timezone in one column', async () => {
    render(<ConfirmProvider><ToastProvider><ProxiesPage onChanged={vi.fn()} /></ToastProvider></ConfirmProvider>)
    const ip = await screen.findByText('1.2.3.4')
    expect(ip.closest('td')?.textContent).toContain('America/New_York')
    expect(screen.getByRole('columnheader', { name: t.proxies.colExit })).toBeTruthy()
    expect(screen.getAllByRole('columnheader')).toHaveLength(6)
    expect(screen.getAllByText(t.proxies.unchecked).length).toBeGreaterThan(0)
  })

  it('warns when sites with IPv6 see the proxy in another timezone', async () => {
    render(<ConfirmProvider><ToastProvider><ProxiesPage onChanged={vi.fn()} /></ToastProvider></ConfirmProvider>)
    const warn = await screen.findByTitle(t.proxies.ipv6Elsewhere('2001:db8::7', 'America/Chicago'))
    expect(warn.closest('td')?.textContent).toContain('5.6.7.8')
    expect(screen.getAllByTitle(/IPv6/)).toHaveLength(1)
  })

  it('lists every provider under Buy proxies and opens one in the browser', async () => {
    render(<ConfirmProvider><ToastProvider><ProxiesPage onChanged={vi.fn()} /></ToastProvider></ConfirmProvider>)
    await screen.findByText('1.2.3.4')
    fireEvent.click(screen.getByRole('button', { name: t.proxies.buy }))
    const dialog = screen.getByRole('dialog', { name: t.proxies.buy })
    for (const p of partnersOf('proxy')) expect(within(dialog).getByText(p.name)).toBeTruthy()
    expect(within(dialog).queryByText('PostClaw')).toBeNull()
    fireEvent.click(within(dialog).getByRole('button', { name: /IPRoyal/ }))
    expect(openExternal).toHaveBeenCalledWith('https://iproyal.com/')
  })

  it('narrows Buy proxies to one proxy type', async () => {
    render(<ConfirmProvider><ToastProvider><ProxiesPage onChanged={vi.fn()} /></ToastProvider></ConfirmProvider>)
    await screen.findByText('1.2.3.4')
    fireEvent.click(screen.getByRole('button', { name: t.proxies.buy }))
    const dialog = screen.getByRole('dialog', { name: t.proxies.buy })
    fireEvent.click(within(within(dialog).getByRole('group', { name: t.partners.filter })).getByRole('button', { name: t.partners.types.mobile }))
    const mobile = partnersOf('proxy').filter((p) => p.types?.includes('mobile'))
    expect(within(dialog).getAllByRole('listitem')).toHaveLength(mobile.length)
    expect(within(dialog).queryByText('Massive')).toBeNull()
  })
})
