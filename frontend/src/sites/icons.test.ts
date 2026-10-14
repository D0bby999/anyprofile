// SPDX-License-Identifier: AGPL-3.0-or-later
import { describe, expect, it } from 'vitest'
import catalog from '../../../internal/site/catalog.json'
import { siteIcons } from './icons.gen'

describe('siteIcons', () => {
  it('has a path and a viewBox for every icon the catalog names', () => {
    for (const { id, icon } of catalog as { id: string; icon: string }[]) {
      if (!icon) continue
      expect(siteIcons[icon], id).toBeDefined()
      expect(siteIcons[icon].viewBox.split(' ').map(Number).every((n) => !Number.isNaN(n)), id).toBe(true)
    }
  })
})
