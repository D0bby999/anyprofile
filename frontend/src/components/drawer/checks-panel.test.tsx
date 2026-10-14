// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { t } from '../../i18n'
import { ChecksPanel, type CheckResult } from './checks-panel'

afterEach(cleanup)

const ids = ['webdriver', 'webrtc_ip', 'tz_profile', 'langs', 'ua_version', 'platform', 'chrome_height', 'geolocation', 'hwc_pin']
const all = (bad?: string): CheckResult[] => ids.map((id) => id === bad
  ? { id, ok: false, level: 'danger', got: 'Asia/Tokyo', want: 'Asia/Taipei' }
  : { id, ok: true, level: 'warn' })

describe('ChecksPanel', () => {
  it('sums up a clean launch', () => {
    render(<ChecksPanel checks={all()} />)
    expect(screen.getByText(t.checks.allPassed(9))).toBeTruthy()
  })

  it('names what failed and why, with what was measured', () => {
    render(<ChecksPanel checks={all('tz_profile')} />)
    fireEvent.click(screen.getByRole('button', { name: t.checks.someFailed(1, 9) }))
    expect(screen.getByText(t.checks.text.tz_profile)).toBeTruthy()
    expect(screen.getByText(/Asia\/Tokyo/)).toBeTruthy()
  })

  it('explains when nothing ran yet', () => {
    render(<ChecksPanel checks={[]} />)
    expect(screen.getByText(t.checks.never)).toBeTruthy()
  })
})
