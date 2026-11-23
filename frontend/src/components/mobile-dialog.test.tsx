// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { t } from '../i18n'
import type { service } from '../api'

let createDeferred: { resolve: (val: unknown) => void; reject: (err: unknown) => void }
const createMobileDevice = vi.fn((_input?: service.MobileInput) => new Promise((resolve, reject) => { createDeferred = { resolve, reject } }))
const updateMobileDevice = vi.fn().mockResolvedValue({})

vi.mock('../api', async (orig) => ({
  ...(await orig<typeof import('../api')>()),
  Api: {
    CreateMobileDevice: (input: service.MobileInput) => createMobileDevice(input),
    UpdateMobileDevice: (id: string, input: service.MobileInput) => updateMobileDevice(id, input),
    ListProxies: () => Promise.resolve([
      { id: 'px1', label: 'proxy1', country: 'US', exitIp: '1.2.3.4', rotating: false, usedBy: [], assignable: true },
    ]),
  },
}))

const { MobileDialog } = await import('./mobile-dialog')

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('MobileDialog', () => {
  it('creates with a unique name and defaults without opening Advanced', async () => {
    const onSaved = vi.fn()
    const onClose = vi.fn()
    render(<MobileDialog existingNames={[t.mobile.defaultName(1)]} onClose={onClose} onSaved={onSaved} />)

    const nameInput = screen.getByRole('textbox', { name: t.mobile.name }) as HTMLInputElement
    expect(nameInput.value).toBe(t.mobile.defaultName(2))
    expect(screen.getByText(t.mobile.deviceSpecs(3, 4))).toBeTruthy()
    expect(screen.getByText(t.drawer.advanced).parentElement?.hasAttribute('open')).toBe(false)
    // Proxy selector is surfaced directly in the main form without opening Advanced.
    const proxySelect = screen.getByLabelText(t.create.proxy) as HTMLSelectElement
    expect(proxySelect.closest('details')).toBeNull()
    const create = screen.getByRole('button', { name: t.mobile.create }) as HTMLButtonElement
    await waitFor(() => expect(create.disabled).toBe(false))
    fireEvent.click(create)

    expect(createMobileDevice).toHaveBeenCalledWith(expect.objectContaining({ name: t.mobile.defaultName(2), cpus: 3, ramMb: 4096, proxyId: 'auto' }))
    expect(screen.getByRole('progressbar', { name: t.mobile.creatingTitle })).toBeTruthy()
    expect(nameInput.disabled).toBe(true)
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(onClose).not.toHaveBeenCalled()
    createDeferred.resolve({ id: 'new-device' })
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith('new-device'))
  })
})
