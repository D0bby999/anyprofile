// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { t } from '../i18n'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'

const createProfile = vi.fn().mockResolvedValue({})
vi.mock('../api', async (orig) => ({
  ...(await orig<typeof import('../api')>()),
  Api: { CreateProfile: (...a: unknown[]) => createProfile(...a), ListProxies: () => Promise.resolve([]) },
}))
const { NewProfileDialog } = await import('./new-profile-dialog')

afterEach(() => { cleanup(); createProfile.mockClear() })

describe('NewProfileDialog', () => {
  it('creates from a name and Enter, with an automatic proxy and the channel start page', async () => {
    const onCreated = vi.fn()
    render(<NewProfileDialog defaultChannel="tiktok" onClose={vi.fn()} onCreated={onCreated} />)
    const name = screen.getByPlaceholderText(t.create.namePlaceholder)
    fireEvent.change(name, { target: { value: '  Kênh mới  ' } })
    fireEvent.submit(name.closest('form')!)
    await waitFor(() => expect(onCreated).toHaveBeenCalled())
    expect(createProfile.mock.calls[0][0]).toMatchObject({
      name: 'Kênh mới', proxyId: 'auto', channelKind: 'tiktok', startUrl: 'https://www.tiktok.com/tiktokstudio',
    })
  })

  it('switches the start page with the channel until the user edits it', () => {
    render(<NewProfileDialog defaultChannel="tiktok" onClose={vi.fn()} onCreated={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: /YouTube/ }))
    expect(screen.getByDisplayValue('https://studio.youtube.com')).toBeTruthy()
    fireEvent.change(screen.getByDisplayValue('https://studio.youtube.com'), { target: { value: 'https://example.com' } })
    fireEvent.click(screen.getByRole('button', { name: /TikTok/ }))
    expect(screen.getByDisplayValue('https://example.com')).toBeTruthy()
  })

  it('does not submit without a name and closes on Escape', () => {
    const onClose = vi.fn()
    render(<NewProfileDialog defaultChannel="other" onClose={onClose} onCreated={vi.fn()} />)
    expect((screen.getByRole('button', { name: t.create.submit }) as HTMLButtonElement).disabled).toBe(true)
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(onClose).toHaveBeenCalled()
  })
})
