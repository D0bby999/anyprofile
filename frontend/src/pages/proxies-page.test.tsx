// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { t } from '../i18n'
import { ConfirmProvider } from '../components/confirm'
import { ToastProvider } from '../components/toast'

const proxy = (id: string, extra = {}) => ({ id, label: `gw.example:${id}`, scheme: 'http', host: 'gw.example', port: 10001, note: '',
  country: 'US', exitIp: '', timezone: '', status: 'unchecked', checkedAt: '', expiresAt: '', rotating: false, usedBy: [], ...extra })
vi.mock('../api', async (orig) => ({
  ...(await orig<typeof import('../api')>()),
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
})
