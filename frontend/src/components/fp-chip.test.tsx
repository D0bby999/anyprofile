// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { service } from '../api'
import { FpChip } from './fp-chip'

const row = (over: Partial<service.ProfileRow>) => service.ProfileRow.createFrom({
  id: 'a', name: 'A', site: 'other', serial: 1, personaId: 'macos-m4pro-1728x1117', login: '',
  hasPassword: false, hasTotp: false, fpVerdict: '', fpDiff: [], fpCheckedAt: '', state: 'stopped',
  stateError: '', cdpPort: 0, tags: [], createdAt: '', lastOpenAt: '', ...over,
})

afterEach(cleanup)

describe('FpChip', () => {
  it('shows the changed chip with a diff title', () => {
    render(<FpChip row={row({ fpVerdict: 'drift', fpDiff: ['canvas', 'screen'] })} />)
    const chip = screen.getByText('Fingerprint changed')
    expect(chip).toBeTruthy()
    expect(chip.closest('.chip')?.getAttribute('title')).toContain('canvas, screen')
  })

  it('renders nothing when unset', () => {
    const { container } = render(<FpChip row={row({})} />)
    expect(container.textContent).toBe('')
  })
})
