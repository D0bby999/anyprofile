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
  it('shows progress loading indicator with steps when creating device', async () => {
    const onSaved = vi.fn()
    render(<MobileDialog row={null} onClose={vi.fn()} onSaved={onSaved} />)

    const nameInput = screen.getByPlaceholderText(t.mobile.namePlaceholder)
    fireEvent.change(nameInput, { target: { value: 'Pixel Test' } })

    const submitBtn = screen.getByRole('button', { name: t.mobile.create })
    fireEvent.click(submitBtn)

    // Should now be busy
    await waitFor(() => {
      expect(screen.getByText(t.mobile.creatingTitle)).toBeTruthy()
    })

    // Inputs should be disabled during creation
    expect(nameInput.hasAttribute('disabled')).toBe(true)

    // Step indicators should be present
    expect(screen.getByText(t.mobile.creatingStep1)).toBeTruthy()
    expect(screen.getByText(t.mobile.creatingStep2)).toBeTruthy()
    expect(screen.getByText(t.mobile.creatingStep3)).toBeTruthy()
    expect(screen.getByText(t.mobile.creatingStep4)).toBeTruthy()

    // Resolve creation
    createDeferred.resolve({ id: 'dev1' })
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith('Pixel Test'))
  })
})
