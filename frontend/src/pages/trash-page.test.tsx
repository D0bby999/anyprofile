// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { t } from '../i18n'
import { ConfirmProvider } from '../components/confirm'
import { ToastProvider } from '../components/toast'

vi.mock('../api', async (orig) => ({
  ...(await orig<typeof import('../api')>()),
  onDataChanged: () => () => {},
  Api: {
    Sites: () => Promise.resolve([]),
    ListProfiles: () => Promise.resolve([]),
    ListMobileDevices: () => Promise.resolve([{ id: 'd1', name: 'Phone 1', serial: 1, ramMb: 3072, deletedAt: '2026-10-24T08:00:00Z' }]),
  },
}))
const { TrashPage } = await import('./trash-page')

afterEach(cleanup)

describe('TrashPage', () => {
  it('counts trashed mobile devices with the profiles', async () => {
    render(<ConfirmProvider><ToastProvider><TrashPage onChanged={vi.fn()} /></ToastProvider></ConfirmProvider>)
    expect(await screen.findByText('Phone 1')).toBeTruthy()
    expect(screen.getByText(t.trash.total(0, 1))).toBeTruthy()
  })
})
