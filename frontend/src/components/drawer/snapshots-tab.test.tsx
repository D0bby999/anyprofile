// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { t } from '../../i18n'

const restore = vi.fn().mockResolvedValue(9)
vi.mock('../../api', async (orig) => ({
  ...(await orig<typeof import('../../api')>()),
  onDataChanged: () => () => {},
  Api: {
    Snapshots: () => Promise.resolve([{ id: 4, profileId: 'a', createdAt: '2026-10-12T21:04:00Z', reason: 'close', bytes: 2048, file: 'f' }]),
    RestoreSnapshot: (...a: unknown[]) => restore(...a),
    TakeSnapshot: () => Promise.resolve({}),
    Storage: () => Promise.resolve({ profilesBytes: 1024, snapshotsBytes: 5 * 1048576, orphans: [] }),
    SnapshotSettings: () => Promise.resolve({ auto: true, keep: 5 }),
  },
}))
const { SnapshotsTab } = await import('./snapshots-tab')
const { StorageSettings } = await import('../storage-settings')
const { ConfirmProvider } = await import('../confirm')
const { ToastProvider } = await import('../toast')

const wrap = (ui: React.ReactNode) => render(<ToastProvider><ConfirmProvider>{ui}</ConfirmProvider></ToastProvider>)

afterEach(() => { cleanup(); restore.mockClear() })

describe('SnapshotsTab', () => {
  it('locks Take snapshot while the profile runs', async () => {
    wrap(<SnapshotsTab profileId="a" running />)
    const take = await screen.findByRole('button', { name: t.snapshots.take })
    expect((take as HTMLButtonElement).disabled).toBe(true)
  })

  it('restores after confirming and offers Undo', async () => {
    wrap(<SnapshotsTab profileId="a" running={false} />)
    fireEvent.click(await screen.findByRole('button', { name: /^Actions for the snapshot/ }))
    fireEvent.click(screen.getByRole('menuitem', { name: t.snapshots.restore }))
    const dialog = await screen.findByRole('dialog')
    fireEvent.click(within(dialog).getByRole('button', { name: 'Restore' }))
    await waitFor(() => expect(restore).toHaveBeenCalledWith(4))
    fireEvent.click(await screen.findByRole('button', { name: t.snapshots.undo }))
    await waitFor(() => expect(restore).toHaveBeenLastCalledWith(9))
  })
})

describe('StorageSettings', () => {
  it('shows how much the snapshots use', async () => {
    wrap(<StorageSettings />)
    expect(await screen.findByText(t.snapshots.total('5.0 MB'))).toBeTruthy()
  })
})
