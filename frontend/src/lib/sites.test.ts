// SPDX-License-Identifier: AGPL-3.0-or-later
import { describe, expect, it } from 'vitest'
import { site } from '../api'
import { glyphColors, guessSite, regionStartUrls, siteById, syncStartUrls, type Site } from './sites'

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

describe('start pages by country', () => {
  const shop = mk('shop', ['seller.example'], { startUrl: 'https://seller.example', startUrls: { US: 'https://seller-us.example', VN: 'https://seller-vn.example' } })
  const yt = mk('youtube', ['youtube.com'], { startUrl: 'https://studio.youtube.com' })
  const all = [shop, yt]

  it("adds the country's page, or the default one, and removes any country's page", () => {
    expect(syncStartUrls('', [], ['shop'], all, 'us')).toBe('https://seller-us.example')
    expect(syncStartUrls('', [], ['shop'], all, 'JP')).toBe('https://seller.example')
    expect(syncStartUrls('https://seller-vn.example\nhttps://mine.example/', ['shop'], ['youtube'], all, 'US'))
      .toBe('https://mine.example/\nhttps://studio.youtube.com')
  })

  it('moves the pages to a new country and keeps typed lines', () => {
    expect(regionStartUrls('https://seller-vn.example\nhttps://mine.example/', ['shop'], all, 'US')).toBe('https://seller-us.example\nhttps://mine.example/')
    expect(regionStartUrls('https://seller-vn.example', [], all, 'US')).toBe('https://seller-vn.example')
  })
})
