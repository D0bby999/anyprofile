// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { ToastProvider, useToast } from './toast'

let api: ReturnType<typeof useToast>
function Grab() { api = useToast(); return null }
const mount = () => render(<ToastProvider><Grab /></ToastProvider>)

beforeEach(() => { vi.useFakeTimers() })
afterEach(() => { cleanup(); vi.useRealTimers() })

describe('toasts', () => {
  it('lets a success toast leave after 4 seconds', () => {
    mount()
    act(() => api.success('Saved'))
    expect(screen.getByText('Saved')).toBeTruthy()
    act(() => { vi.advanceTimersByTime(4100) })
    expect(screen.queryByText('Saved')).toBeNull()
  })

  it('keeps an error until it is dismissed, and runs its action', () => {
    mount()
    const run = vi.fn()
    act(() => api.error('Engine missing', { label: 'Install', run }))
    act(() => { vi.advanceTimersByTime(10000) })
    expect(screen.getByRole('alert').textContent).toContain('Engine missing')
    fireEvent.click(screen.getByRole('button', { name: 'Install' }))
    expect(run).toHaveBeenCalled()
    expect(screen.queryByText('Engine missing')).toBeNull()
  })

  it('drops only the errors when the screen changes', () => {
    mount()
    act(() => { api.error('Proxy not answering'); api.info('Checking') })
    act(() => api.clearErrors())
    expect(screen.queryByText('Proxy not answering')).toBeNull()
    expect(screen.getByText('Checking')).toBeTruthy()
  })

  it('keeps at most three', () => {
    mount()
    act(() => { api.error('one'); api.error('two'); api.error('three'); api.error('four') })
    expect(screen.queryByText('one')).toBeNull()
    expect(screen.getAllByRole('alert')).toHaveLength(3)
  })
})
