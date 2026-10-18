// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { t } from '../i18n'

const exportBackup = vi.fn()
const pickBackup = vi.fn()
const importBackup = vi.fn()
vi.mock('../api', async (orig) => ({
  ...(await orig<typeof import('../api')>()),
  Api: {
    ExportBackup: (pw: string) => exportBackup(pw),
    PickBackup: () => pickBackup(),
    ImportBackup: (pw: string) => importBackup(pw),
  },
}))
const { BackupSettings, BackupRestore } = await import('./backup-settings')
const result = (over = {}) => ({ profiles: 0, proxies: 0, accounts: 0, extensions: 0, bytes: 0, profilesHere: 0, proxiesHere: 0, ...over })

afterEach(() => { cleanup(); vi.clearAllMocks() })

describe('BackupSettings', () => {
  it('saves only with a long enough password typed twice', async () => {
    exportBackup.mockResolvedValue(result({ profiles: 3, proxies: 2, bytes: 2048 }))
    render(<BackupSettings />)
    const save = screen.getByRole('button', { name: t.backup.save }) as HTMLButtonElement
    const [pw, again] = [screen.getByLabelText(t.backup.password), screen.getByLabelText(t.backup.again)]
    fireEvent.change(pw, { target: { value: 'short' } })
    fireEvent.change(again, { target: { value: 'short' } })
    expect(save.disabled).toBe(true)
    fireEvent.change(pw, { target: { value: 'long enough' } })
    fireEvent.change(again, { target: { value: 'long enougH' } })
    expect(screen.getByText(t.backup.mismatch)).toBeTruthy()
    expect(save.disabled).toBe(true)
    fireEvent.change(again, { target: { value: 'long enough' } })
    fireEvent.click(save)
    await screen.findByText(t.backup.saved(3, 2, '2 KB'))
    expect(exportBackup).toHaveBeenCalledWith('long enough')
    expect((screen.getByLabelText(t.backup.password) as HTMLInputElement).value).toBe('')
  })

  it('says nothing when the save dialog is cancelled', async () => {
    exportBackup.mockResolvedValue(result())
    render(<BackupSettings />)
    fireEvent.change(screen.getByLabelText(t.backup.password), { target: { value: 'long enough' } })
    fireEvent.change(screen.getByLabelText(t.backup.again), { target: { value: 'long enough' } })
    fireEvent.click(screen.getByRole('button', { name: t.backup.save }))
    await waitFor(() => expect(exportBackup).toHaveBeenCalled())
    expect(screen.queryByRole('status')).toBeNull()
  })
})

describe('BackupRestore', () => {
  it('picks a file, asks its password, and says what was added', async () => {
    pickBackup.mockResolvedValue('AnyProfile-2026-10-18.apbackup')
    importBackup.mockRejectedValueOnce(new Error('anyprofile:backup_wrong_password: wrong password'))
    importBackup.mockResolvedValueOnce(result({ profiles: 2, proxies: 1, profilesHere: 1 }))
    const onRestored = vi.fn()
    render(<BackupRestore onRestored={onRestored} />)
    fireEvent.click(screen.getByRole('button', { name: t.backup.pick }))
    expect(await screen.findByText('AnyProfile-2026-10-18.apbackup')).toBeTruthy()
    fireEvent.change(screen.getByLabelText(t.backup.passwordOf), { target: { value: 'nope' } })
    fireEvent.click(screen.getByRole('button', { name: t.backup.restore }))
    expect(await screen.findByText(t.errors.backup_wrong_password)).toBeTruthy()
    expect(onRestored).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: t.backup.restore }))
    await screen.findByText(t.backup.restored(2, 1), { exact: false })
    expect(screen.getByText(t.backup.alreadyHere(1, 0), { exact: false })).toBeTruthy()
    expect(onRestored).toHaveBeenCalledWith(expect.objectContaining({ profiles: 2 }))
    expect(importBackup).toHaveBeenLastCalledWith('nope')
  })
})
