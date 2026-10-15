// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { t } from '../../i18n'
import { ConfirmProvider } from '../confirm'
import { ToastProvider } from '../toast'

const resetFingerprint = vi.fn().mockResolvedValue(undefined)
vi.mock('../../api', async (orig) => ({
  ...(await orig<typeof import('../../api')>()),
  Api: { ResetFingerprint: (id: string) => resetFingerprint(id) },
}))
const { IdentityGroup } = await import('./identity-group')

const now = new Date().toISOString()
const row = { id: 'a', name: 'AI', sites: [], serial: 3, engine: 'apostate', engineVersion: '0.5.1', personaId: 'macos-m3pro-1728x1117',
  seedHex: '1d669920771a0e2b', createdAt: now, fpCheckedAt: now, fpVerdict: 'baseline', fpDiff: [], state: 'stopped', proxyId: 'px', lastOpenAt: now }

afterEach(cleanup)

describe('IdentityGroup', () => {
  it('shows the fingerprint check time and measures again after confirming', async () => {
    render(<ConfirmProvider><ToastProvider><IdentityGroup row={row as never} /></ToastProvider></ConfirmProvider>)
    const reset = screen.getByRole('button', { name: t.fp.reset })
    expect(reset.closest('.fpmeta')?.textContent).toContain(t.drawer.fingerprintChecked(''))
    fireEvent.click(reset)
    fireEvent.click(await screen.findByRole('button', { name: t.confirm.resetFingerprint.confirm }))
    await waitFor(() => expect(resetFingerprint).toHaveBeenCalledWith('a'))
  })
})
