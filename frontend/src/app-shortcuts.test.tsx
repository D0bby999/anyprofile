// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { t } from './i18n'

const profile = {
  id: 'tw', name: 'Thử proxy TW', sites: ['other'], serial: 1, engine: 'fury', engineVersion: '0.3.5', personaId: 'macos-m1pro-1512x982',
  machine: '', proxyId: '', state: 'stopped', cdpPort: 0, tags: [], notes: '', login: '', lastOpenAt: '', fpVerdict: '', fpDiff: [],
  startUrl: '', seedHex: '00112233aabbccdd', createdAt: '2026-10-12T00:00:00Z', timezone: 'Asia/Taipei', languages: 'zh-TW',
}
const flags = { onboarded: true, created: true }
const api: Record<string, (...a: unknown[]) => Promise<unknown>> = {
  UpdateStatus: () => Promise.resolve(null),
  ListProfiles: () => Promise.resolve(flags.created ? [profile] : []),
  CreateProfiles: () => { flags.created = true; return Promise.resolve([{ id: 'tw' }]) },
  FinishOnboarding: () => Promise.resolve(),
  Defaults: () => Promise.resolve({ engine: 'apostate', site: 'other', proxy: 'auto' }),
  Engines: () => Promise.resolve([{ engine: 'apostate', version: '0.5.1', chromium: '155', installed: true, problem: '', default: true }]),
  StartupError: () => Promise.resolve(''),
  Onboarded: () => Promise.resolve(flags.onboarded),
  FileVaultOn: () => Promise.resolve(true),
}
vi.mock('./api', async (orig) => ({
  ...(await orig<typeof import('./api')>()),
  onDataChanged: () => () => {},
  onMenu: () => () => {},
  onLaunchChanged: () => () => {},
  onEngineProgress: () => () => {},
  Api: new Proxy({}, { get: (_, name: string) => api[name] ?? (() => Promise.resolve([])) }),
}))
// jsdom has no matchMedia; the app reads it to follow the system theme.
window.matchMedia ??= ((q: string) => ({ matches: false, media: q, addEventListener() {}, removeEventListener() {} })) as unknown as typeof window.matchMedia
const { default: App } = await import('./App')

afterEach(() => { cleanup(); flags.onboarded = true; flags.created = true })

describe('App shortcuts', () => {
  it('opens the palette with ⌘K and a profile from it with Enter', async () => {
    render(<App />)
    await screen.findByText('Thử proxy TW')
    fireEvent.keyDown(document.body, { key: 'k', metaKey: true })
    const input = await screen.findByRole('combobox', { name: t.palette.title })
    fireEvent.change(input, { target: { value: 'thu' } })
    expect(screen.getByRole('option', { name: /Thử proxy TW/ })).toBeTruthy()
    fireEvent.keyDown(input, { key: 'Enter' })
    await waitFor(() => expect(screen.getByRole('complementary', { name: t.drawer.label('Thử proxy TW') })).toBeTruthy())
  })

  it('opens New profile with ⌘N and the shortcut list with ?', async () => {
    render(<App />)
    await screen.findByText('Thử proxy TW')
    fireEvent.keyDown(document.body, { key: 'n', metaKey: true })
    expect(await screen.findByRole('dialog', { name: t.create.title })).toBeTruthy()
    fireEvent.keyDown(window, { key: 'Escape' })
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    fireEvent.keyDown(document.body, { key: '?' })
    expect(await screen.findByRole('dialog', { name: t.keys.title })).toBeTruthy()
  })

  it('says when nothing matches', async () => {
    render(<App />)
    await screen.findByText('Thử proxy TW')
    fireEvent.keyDown(document.body, { key: 'k', metaKey: true })
    fireEvent.change(await screen.findByRole('combobox', { name: t.palette.title }), { target: { value: 'zzz' } })
    expect(screen.getByText(t.palette.none('zzz'))).toBeTruthy()
  })

  it('shows first-run setup only until onboarding is done', async () => {
    flags.onboarded = false
    render(<App />)
    expect(await screen.findByRole('heading', { name: t.welcome.title })).toBeTruthy()
    cleanup()
    flags.onboarded = true
    render(<App />)
    await screen.findByText('Thử proxy TW')
    expect(screen.queryByRole('heading', { name: t.welcome.title })).toBeNull()
  })

  it('opens the new profile after setup, even before the list has it', async () => {
    flags.onboarded = false
    flags.created = false
    render(<App />)
    fireEvent.click(await screen.findByRole('button', { name: t.welcome.skipProxy }))
    fireEvent.click(screen.getByRole('button', { name: t.welcome.continueAnyway }))
    fireEvent.change(screen.getByPlaceholderText(t.welcome.namePlaceholder), { target: { value: 'Thử proxy TW' } })
    fireEvent.click(screen.getByRole('button', { name: t.welcome.create }))
    await waitFor(() => expect(screen.getByRole('complementary', { name: t.drawer.label('Thử proxy TW') })).toBeTruthy())
  })
})
