// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, expect, it, vi } from 'vitest'
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import type { MobileInfo } from '../api'

let changed: (info: MobileInfo) => void = () => {}
vi.mock('../api', async (orig) => ({
  ...(await orig<typeof import('../api')>()),
  Api: { ListMobileDevices: () => Promise.resolve([{ id: 'd1', state: 'stopped' }]), MobileAvailability: () => Promise.resolve(null) },
  onDataChanged: () => () => {},
  onMobileChanged: (fn: typeof changed) => { changed = fn; return () => {} },
}))
const { useMobile } = await import('./use-mobile')
afterEach(cleanup)

it('tracks and clears startup phases from mobile events', async () => {
  const { result } = renderHook(() => useMobile())
  await waitFor(() => expect(result.current.rows).toHaveLength(1))
  act(() => changed({ deviceId: 'd1', state: 'starting', phase: 'boot' }))
  expect(result.current.phases.d1).toBe('boot')
  expect(result.current.rows[0].state).toBe('starting')
  act(() => changed({ deviceId: 'd1', state: 'running' }))
  expect(result.current.phases.d1).toBe('')
})
