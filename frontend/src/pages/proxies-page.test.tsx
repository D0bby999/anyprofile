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
    proxy('3', { exitIp: '5.6.7.8', timezone: 'America/New_York', exitIp6: '2001:db8::7', timezone6: 'America/Chicago' }),
    proxy('4', { exitIp: '9.9.9.9', timezone: 'Europe/Paris', checkedAt: '2026-10-24T08:00:00Z', failedAt: '2026-10-24T08:00:00Z' }),
    proxy('5', { checkedAt: '2026-10-24T08:00:00Z', failedAt: '2026-10-24T08:00:00Z' })]),
    CheckAllProxies: () => Promise.resolve(2),
    ImportProxies: () => Promise.resolve({ added: 0, failed: [{ line: 1, err: 'expected host:port or host:port:user:pass', code: 'format', display: '' }], unreachable: [] }) },
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

  it('marks a proxy whose last check failed, and says how many failed after Check all', async () => {
    render(<ConfirmProvider><ToastProvider><ProxiesPage onChanged={vi.fn()} /></ToastProvider></ConfirmProvider>)
    const ip = await screen.findByText('9.9.9.9')
    expect(within(ip.closest('tr')!).getByText(t.proxies.failed)).toBeTruthy()
    expect(within(ip.closest('tr')!).queryByText(t.proxies.ok)).toBeNull()
    // Never answered: no exit to show, but it was checked.
    const never = screen.getByText('gw.example:5').closest('tr')!
    expect(within(never).getByText(t.proxies.failed)).toBeTruthy()
    expect(within(never).queryByText(t.proxies.unchecked)).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: t.proxies.checkAll }))
    expect(await screen.findByText(t.proxies.failedCount(2))).toBeTruthy()
  })

  it('puts a pasted line that is not a proxy in words', async () => {
    render(<ConfirmProvider><ToastProvider><ProxiesPage onChanged={vi.fn()} /></ToastProvider></ConfirmProvider>)
    await screen.findByText('1.2.3.4')
    fireEvent.change(screen.getByRole('textbox', { name: t.proxies.addTitle }), { target: { value: 'proxy cua toi' } })
    fireEvent.click(screen.getByRole('button', { name: t.proxies.import }))
    expect(await screen.findByText(t.proxies.failedLine(1, t.proxies.parseErr.format))).toBeTruthy()
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
