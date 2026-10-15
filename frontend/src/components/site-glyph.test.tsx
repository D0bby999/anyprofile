// SPDX-License-Identifier: AGPL-3.0-or-later
// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, render } from '@testing-library/react'
import { site } from '../api'
import { SiteBadges } from './site-glyph'

const mk = (id: string, name: string) => site.Site.createFrom({ id, name, group: 'other', domains: [], startUrl: '', icon: '', color: '#6B7280', custom: false })
const sites = [mk('tiktok', 'TikTok'), mk('youtube', 'YouTube'), mk('gmail', 'Gmail'), mk('etsy', 'Etsy'), mk('amazon', 'Amazon'), mk('other', 'Other')]

afterEach(cleanup)

describe('SiteBadges', () => {
  it('shows at most three icons and counts every platform in the label and tooltip', () => {
    const { container } = render(<SiteBadges ids={['tiktok', 'youtube', 'gmail', 'etsy', 'amazon']} sites={sites} />)
    expect(container.querySelectorAll('.glyph')).toHaveLength(3)
    expect(container.querySelector('.plabel')?.textContent).toBe('TikTok +4')
    expect(container.querySelector('.platform')?.getAttribute('title')).toBe('TikTok, YouTube, Gmail, Etsy, Amazon')
  })

  it('shows every icon up to three', () => {
    const { container } = render(<SiteBadges ids={['tiktok', 'gmail']} sites={sites} />)
    expect(container.querySelectorAll('.glyph')).toHaveLength(2)
  })
})
