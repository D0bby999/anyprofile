// SPDX-License-Identifier: AGPL-3.0-or-later
import { describe, expect, it } from 'vitest'
import { site } from '../api'
import { glyphColors, guessSite, siteById, type Site } from './sites'

const mk = (id: string, domains: string[], over: Partial<Site> = {}): Site =>
  site.Site.createFrom({ id, name: id, group: 'other', domains, startUrl: '', icon: '', color: '#6B7280', custom: false, ...over })

const sites = [
  mk('facebook', ['facebook.com']),
  mk('facebook-ads', ['business.facebook.com', 'ads.facebook.com']),
  mk('etsy', ['etsy.com']),
  mk('other', []),
]

describe('guessSite', () => {
  it('prefers the longest matching domain', () => {
    expect(guessSite('https://business.facebook.com/x', sites)).toBe('facebook-ads')
  })
  it('matches plain hosts and strips www', () => {
    expect(guessSite('https://www.etsy.com/your/shops/me', sites)).toBe('etsy')
  })
  it('returns null for a non-URL', () => {
    expect(guessSite('not a url', sites)).toBeNull()
  })
})

describe('glyphColors', () => {
  it('uses dark text on bright backgrounds', () => {
    expect(glyphColors('#FFE600').color).toBe('#111')
  })
  it('uses white text on dark backgrounds', () => {
    expect(glyphColors('#000000').color).toBe('#fff')
  })
})

describe('siteById', () => {
  it('finds, falls back to other, then to a neutral badge', () => {
    expect(siteById(sites, 'etsy').name).toBe('etsy')
    expect(siteById(sites, 'nope').id).toBe('other')
    expect(siteById([], 'nope').color).toBe('#6B7280')
  })
})
