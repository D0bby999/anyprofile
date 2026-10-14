// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen, within } from '@testing-library/react'
import { t } from '../../i18n'

let sessions: unknown[] = []
vi.mock('../../api', async (orig) => ({
  ...(await orig<typeof import('../../api')>()),
  onDataChanged: () => () => {},
  Api: { Sessions: () => Promise.resolve(sessions) },
}))
const { SessionsTab } = await import('./sessions-tab')

const s = (id: number, ip: string, cc: string, stopped = true) => ({
  id, profileId: 'a', engine: 'apostate', engineVersion: '0.5.1', proxyId: 'px', startedAt: '2026-10-13T10:00:00Z',
  stoppedAt: stopped ? '2026-10-13T10:30:00Z' : '', exitIp: ip, exitCountry: cc, exitTz: '', exitError: '', checks: [],
})

afterEach(() => { cleanup(); sessions = [] })

describe('SessionsTab', () => {
  it('lists one row per session and marks a country change', async () => {
    sessions = [s(3, '2.2.2.2', 'DE', false), s(2, '1.1.1.1', 'US'), s(1, '1.1.1.1', 'US')]
    render(<SessionsTab profileId="a" />)
    const rows = await screen.findAllByRole('row')
    expect(rows).toHaveLength(4) // header and three sessions
    expect(within(rows[1]).getByText(t.flags.country_changed as string)).toBeTruthy()
    expect(within(rows[1]).getByText(t.sessions.running)).toBeTruthy()
    expect(within(rows[2]).queryByText(t.flags.country_changed as string)).toBeNull()
  })

  it('says how to start when there is none', async () => {
    render(<SessionsTab profileId="a" />)
    expect(await screen.findByText(t.sessions.empty)).toBeTruthy()
  })
})
