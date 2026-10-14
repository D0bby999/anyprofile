// SPDX-License-Identifier: AGPL-3.0-or-later
import { describe, expect, it } from 'vitest'
import { t } from '../i18n'
import { health } from './health'

const base = { fpVerdict: '', fpDiff: [] as string[], proxyExpiry: '', lastOpenAt: '2026-10-13T10:00:00Z', cdpPort: 0 }

describe('health', () => {
  it.each([
    [{ fpVerdict: 'drift', fpDiff: ['canvas', 'screen'] }, 'danger', t.health.drift],
    [{ proxyExpiry: 'expired' }, 'danger', t.health.expired],
    [{ proxyExpiry: 'soon' }, 'warn', t.health.expiring],
    [{ fpVerdict: 'unchecked' }, 'warn', t.health.unchecked],
    [{ fpVerdict: 'baseline' }, 'info', t.health.baseline],
    [{ fpVerdict: 'match' }, 'ok', t.health.ok],
    [{ lastOpenAt: '' }, 'none', t.health.new],
  ])('%o is %s', (over, level, label) => {
    const h = health({ ...base, ...over })
    expect(h.level).toBe(level)
    expect(h.label).toBe(label)
    expect(h.reasons).toHaveLength(1)
  })

  it('names the changed values in words', () => {
    expect(health({ ...base, fpVerdict: 'drift', fpDiff: ['canvas', 'screen'] }).reasons[0])
      .toBe(t.health.driftText('canvas, screen'))
  })

  it('keeps every reason and shows the worst', () => {
    const h = health({ ...base, fpVerdict: 'match', proxyExpiry: 'expired' })
    expect(h.level).toBe('danger')
    expect(h.label).toBe(t.health.expired)
    expect(h.reasons).toEqual([t.health.expiredText, t.health.okText])
  })

  it('reports drift and an expired proxy together', () => {
    const h = health({ ...base, fpVerdict: 'drift', fpDiff: ['tz'], proxyExpiry: 'expired' })
    expect(h.level).toBe('danger')
    expect(h.reasons).toHaveLength(2)
  })
})
