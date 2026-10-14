// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { t } from '../i18n'

let saved = { engine: 'apostate', site: 'other', proxy: 'auto', onOpen: 'restore' }
const setDefaults = vi.fn(async (d: typeof saved) => { saved = { ...d } })
vi.mock('../api', async (orig) => ({
  ...(await orig<typeof import('../api')>()),
  Api: {
    Defaults: () => Promise.resolve({ ...saved }),
    SetDefaults: (d: typeof saved) => setDefaults(d),
  },
}))
const { OnOpenSettings } = await import('./defaults-settings')

afterEach(() => { cleanup(); setDefaults.mockClear(); saved = { engine: 'apostate', site: 'other', proxy: 'auto', onOpen: 'restore' } })

describe('OnOpenSettings', () => {
  it('saves the choice and explains it', async () => {
    render(<OnOpenSettings />)
    expect(screen.getByText(t.onOpen.restoreHint)).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: t.onOpen.start }))
    await waitFor(() => expect(setDefaults).toHaveBeenCalled())
    expect(setDefaults.mock.calls[0][0]).toMatchObject({ onOpen: 'start' })
    expect(screen.getByText(t.onOpen.startHint)).toBeTruthy()
  })

  it('keeps a value another section saved meanwhile', async () => {
    render(<OnOpenSettings />)
    await waitFor(() => expect(screen.getByRole('button', { name: t.onOpen.restore }).getAttribute('aria-pressed')).toBe('true'))
    saved = { ...saved, site: 'etsy' } // changed elsewhere after this section loaded
    fireEvent.click(screen.getByRole('button', { name: t.onOpen.start }))
    await waitFor(() => expect(setDefaults).toHaveBeenCalled())
    expect(setDefaults.mock.calls[0][0]).toMatchObject({ site: 'etsy', onOpen: 'start' })
  })
})
