// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { t } from '../i18n'

const api = { CopyDiagnostics: vi.fn(() => Promise.resolve()) }
vi.mock('../api', async (orig) => ({ ...(await orig<typeof import('../api')>()), Api: api }))
const { SupportSettings } = await import('./support-settings')
const { ToastProvider } = await import('./toast')

afterEach(cleanup)

describe('SupportSettings', () => {
  it('copies the diagnostics through Go and says so', async () => {
    render(<ToastProvider><SupportSettings /></ToastProvider>)
    fireEvent.click(screen.getByRole('button', { name: t.support.copy }))
    await waitFor(() => expect(api.CopyDiagnostics).toHaveBeenCalledOnce())
    expect(await screen.findByText(t.support.copied)).toBeTruthy()
  })
})
