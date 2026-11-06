// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { t } from '../i18n'

let saved = { engine: 'anybrowser', site: 'other', proxy: 'auto', onOpen: 'start' }
let savedMax = 0
const setDefaults = vi.fn(async (d: typeof saved) => { saved = { ...d } })
const setMax = vi.fn(async (n: number) => { savedMax = n })
vi.mock('../api', async (orig) => ({
  ...(await orig<typeof import('../api')>()),
  onDataChanged: () => () => {},
  Api: {
    Defaults: () => Promise.resolve({ ...saved }),
    SetDefaults: (d: typeof saved) => setDefaults(d),
    MaxConcurrentLaunches: () => Promise.resolve(savedMax),
    SetMaxConcurrentLaunches: (n: number) => setMax(n),
  },
}))
const { OnOpenSettings } = await import('./defaults-settings')

afterEach(() => { cleanup(); setDefaults.mockClear(); setMax.mockClear(); saved = { engine: 'anybrowser', site: 'other', proxy: 'auto', onOpen: 'start' }; savedMax = 0 })

describe('OnOpenSettings', () => {
  it('opens the start pages by default, and saves the choice to reopen last tabs', async () => {
    render(<OnOpenSettings />)
    expect(screen.getByRole('button', { name: t.onOpen.start }).getAttribute('aria-pressed')).toBe('true')
    expect(screen.getByText(t.onOpen.startHint)).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: t.onOpen.restore }))
    await waitFor(() => expect(setDefaults).toHaveBeenCalled())
    expect(setDefaults.mock.calls[0][0]).toMatchObject({ onOpen: 'restore' })
    expect(screen.getByText(t.onOpen.restoreHint)).toBeTruthy()
  })

  it('keeps a value another section saved meanwhile', async () => {
    saved = { ...saved, onOpen: 'restore' }
    render(<OnOpenSettings />)
    await waitFor(() => expect(screen.getByRole('button', { name: t.onOpen.restore }).getAttribute('aria-pressed')).toBe('true'))
    saved = { ...saved, site: 'etsy' } // changed elsewhere after this section loaded
    fireEvent.click(screen.getByRole('button', { name: t.onOpen.start }))
    await waitFor(() => expect(setDefaults).toHaveBeenCalled())
    expect(setDefaults.mock.calls[0][0]).toMatchObject({ site: 'etsy', onOpen: 'start' })
  })

  it('loads and saves max concurrent launches', async () => {
    savedMax = 3
    render(<OnOpenSettings />)
    const input = await screen.findByRole('spinbutton', { name: t.onOpen.maxConcurrent })
    expect((input as HTMLInputElement).value).toBe('3')
    fireEvent.change(input, { target: { value: '5' } })
    fireEvent.blur(input)
    await waitFor(() => expect(setMax).toHaveBeenCalledWith(5))
  })
})
