// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { t } from '../i18n'
import { ConfirmProvider } from './confirm'
import { ToastProvider } from './toast'

let status = { enabled: false, port: 51525, running: false, error: '' }
const setAPISettings = vi.fn(async (s: { enabled: boolean; port: number }) => { status = { ...status, ...s, running: s.enabled }; return status })
const copyAPIToken = vi.fn().mockResolvedValue(undefined)
const newAPIToken = vi.fn().mockResolvedValue(undefined)
const copyMCPSetup = vi.fn((_: string) => Promise.resolve())
const openAgentFolder = vi.fn().mockResolvedValue(undefined)
vi.mock('../api', async (orig) => ({
  ...(await orig<typeof import('../api')>()),
  Api: {
    APIStatus: () => Promise.resolve(status),
    SetAPISettings: (s: { enabled: boolean; port: number }) => setAPISettings(s),
    CopyAPIToken: () => copyAPIToken(),
    NewAPIToken: () => newAPIToken(),
    CopyMCPSetup: (c: string) => copyMCPSetup(c),
    OpenAgentFolder: (w: string) => openAgentFolder(w),
  },
}))
const { APISettings } = await import('./api-settings')
const view = () => render(<ConfirmProvider><ToastProvider><APISettings /></ToastProvider></ConfirmProvider>)

afterEach(() => { cleanup(); vi.clearAllMocks(); localStorage.clear(); status = { enabled: false, port: 51525, running: false, error: '' } })

describe('APISettings', () => {
  it('turns the API on and says where it listens', async () => {
    view()
    expect(await screen.findByText(t.api.turnOnFirst)).toBeTruthy()
    fireEvent.click(await screen.findByRole('button', { name: t.api.on }))
    await waitFor(() => expect(setAPISettings).toHaveBeenCalledWith(expect.objectContaining({ enabled: true, port: 51525 })))
    expect(await screen.findByText(t.api.listening(51525))).toBeTruthy()
  })

  it('copies the token through Go and asks before making a new one', async () => {
    status = { enabled: true, port: 51525, running: true, error: '' }
    view()
    fireEvent.click(await screen.findByRole('button', { name: t.api.copyToken }))
    await waitFor(() => expect(copyAPIToken).toHaveBeenCalled())
    fireEvent.click(screen.getByRole('button', { name: t.api.newToken }))
    expect(newAPIToken).not.toHaveBeenCalled()
    fireEvent.click(await screen.findByRole('button', { name: t.api.newTokenConfirm.confirm }))
    await waitFor(() => expect(newAPIToken).toHaveBeenCalled())
  })

  it('copies the setup of the agent chosen, through Go, and says where it goes', async () => {
    status = { ...status, enabled: true, running: true, port: 51600 }
    view()
    expect(await screen.findByText('http://127.0.0.1:51600/mcp')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: new RegExp(t.api.clients.claude.copy) }))
    await waitFor(() => expect(copyMCPSetup).toHaveBeenCalledWith('claude'))
    expect(await screen.findByText(t.api.mcpCopied)).toBeTruthy()
    // Codex reads a config file: the button and the hint say so, and the choice is kept.
    fireEvent.change(screen.getByRole('combobox', { name: t.api.mcpClient }), { target: { value: 'codex' } })
    expect(screen.getByText(t.api.clients.codex.where)).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: new RegExp(t.api.clients.codex.copy) }))
    await waitFor(() => expect(copyMCPSetup).toHaveBeenLastCalledWith('codex'))
    cleanup()
    view()
    expect(((await screen.findByRole('combobox', { name: t.api.mcpClient })) as HTMLSelectElement).value).toBe('codex')
    fireEvent.click(screen.getByRole('button', { name: t.api.openUploads }))
    fireEvent.click(screen.getByRole('button', { name: t.api.openRecipes }))
    expect(openAgentFolder.mock.calls).toEqual([['uploads'], ['recipes']])
  })

  it('explains a busy port', async () => {
    status = { enabled: true, port: 51525, running: false, error: 'anyprofile:api_port_busy: port 51525 is in use' }
    view()
    expect(await screen.findByText(t.errors.api_port_busy)).toBeTruthy()
  })
})
