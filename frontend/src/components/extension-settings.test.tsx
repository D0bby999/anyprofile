// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { t } from '../i18n'
import { ConfirmProvider } from './confirm'
import { ToastProvider } from './toast'

const addExtension = vi.fn().mockResolvedValue({ id: 'ext-2', name: 'New', version: '2', createdAt: '', profiles: 0 })
const deleteExtension = vi.fn().mockResolvedValue(undefined)
vi.mock('../api', async (orig) => ({
  ...(await orig<typeof import('../api')>()),
  onDataChanged: () => () => {},
  Api: {
    ListExtensions: () => Promise.resolve([{ id: 'ext-1', name: 'Helper', version: '1.0', createdAt: '', profiles: 3 }]),
    AddExtension: (folder: boolean) => addExtension(folder),
    DeleteExtension: (id: string) => deleteExtension(id),
  },
}))
const { ExtensionSettings } = await import('./extension-settings')
const view = () => render(<ConfirmProvider><ToastProvider><ExtensionSettings /></ToastProvider></ConfirmProvider>)

afterEach(() => { cleanup(); vi.clearAllMocks() })

describe('ExtensionSettings', () => {
  it('lists the library with how many profiles use each', async () => {
    view()
    expect(await screen.findByText('Helper')).toBeTruthy()
    expect(screen.getByText(t.extensions.usedBy(3))).toBeTruthy()
  })

  it('adds from a file or a folder', async () => {
    view()
    fireEvent.click(screen.getByRole('button', { name: t.extensions.addFile }))
    await waitFor(() => expect(addExtension).toHaveBeenCalledWith(false))
    fireEvent.click(screen.getByRole('button', { name: t.extensions.addFolder }))
    await waitFor(() => expect(addExtension).toHaveBeenCalledWith(true))
  })

  it('asks before deleting, saying how many profiles lose it', async () => {
    view()
    fireEvent.click(await screen.findByRole('button', { name: t.extensions.delete('Helper') }))
    expect(await screen.findByText(t.confirm.deleteExtension('Helper', 3).body)).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: t.confirm.deleteExtension('Helper', 3).confirm }))
    await waitFor(() => expect(deleteExtension).toHaveBeenCalledWith('ext-1'))
  })
})
