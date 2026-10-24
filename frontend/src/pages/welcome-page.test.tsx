// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { t } from '../i18n'
import { site } from '../api'

const state = { installed: false, proxies: [] as unknown[] }
const createProfiles = vi.fn().mockResolvedValue([{ id: 'new1' }])
const finish = vi.fn().mockResolvedValue(undefined)
vi.mock('../api', async (orig) => ({
  ...(await orig<typeof import('../api')>()),
  onEngineProgress: () => () => {},
  Api: {
    Engines: () => Promise.resolve([{ engine: 'apostate', version: '0.5.1', chromium: '155', installed: state.installed, problem: '', default: true }]),
    ListProxies: () => Promise.resolve(state.proxies),
    CreateProfiles: (...a: unknown[]) => createProfiles(...a),
    FinishOnboarding: () => finish(),
  },
}))
const { WelcomePage } = await import('./welcome-page')

const sites = [site.Site.createFrom({ id: 'other', name: 'Other', group: 'other', domains: [], startUrl: '', icon: '', color: '#6B7280', custom: false })]
const page = (onDone = vi.fn()) => render(<WelcomePage sites={sites} fileVaultOn lang="en" onLanguage={vi.fn()} onDone={onDone} />)
const step = (n: number, title: string) => screen.getByRole('region', { name: `${t.welcome.step(n)}: ${title}` })

afterEach(() => { cleanup(); state.installed = false; state.proxies = []; createProfiles.mockClear(); finish.mockClear() })

describe('WelcomePage', () => {
  it('starts at the engine when it is not installed', async () => {
    page()
    await waitFor(() => expect(step(1, t.welcome.engineTitle).className).toContain('active'))
    expect(step(2, t.welcome.proxyTitle).className).toContain('later')
    expect(step(3, t.welcome.profileTitle).className).toContain('later')
  })

  it('goes to the proxy step once the engine is installed', async () => {
    state.installed = true
    page()
    await waitFor(() => expect(step(2, t.welcome.proxyTitle).className).toContain('active'))
    expect(step(1, t.welcome.engineTitle).className).toContain('done')
  })

  it('warns before going on without a proxy', async () => {
    state.installed = true
    page()
    fireEvent.click(await screen.findByRole('button', { name: t.welcome.skipProxy }))
    expect(screen.getByText(t.welcome.noProxyWarn)).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: t.welcome.continueAnyway }))
    expect(step(3, t.welcome.profileTitle).className).toContain('active')
  })

  it('creates the profile, finishes setup and hands over the new id', async () => {
    state.installed = true
    state.proxies = [{ id: 'px', label: 'gw:1', country: 'US', exitIp: '1.1.1.1', timezone: 'America/New_York', rotating: false, usedBy: [], assignable: true }]
    const onDone = vi.fn()
    page(onDone)
    const name = await screen.findByPlaceholderText(t.welcome.namePlaceholder)
    fireEvent.change(name, { target: { value: 'Etsy US' } })
    fireEvent.click(screen.getByRole('button', { name: t.welcome.create }))
    await waitFor(() => expect(onDone).toHaveBeenCalledWith('new1'))
    expect(createProfiles.mock.calls[0][0]).toMatchObject({ baseName: 'Etsy US', count: 1, proxy: 'auto' })
    expect(finish).toHaveBeenCalled()
  })
})
