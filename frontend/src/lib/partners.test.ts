// SPDX-License-Identifier: AGPL-3.0-or-later
import { describe, expect, it } from 'vitest'
import { en } from '../i18n/en'
import { vi as viCatalog } from '../i18n/vi'
import { partnerLink, partnerLogo, partners, partnersOf, proxyTypes, toolGroups } from './partners'

describe('partners', () => {
  it('have unique ids, https links, a bundled logo and a description in both languages', () => {
    expect(new Set(partners.map((p) => p.id)).size).toBe(partners.length)
    for (const p of partners) {
      for (const u of [p.url, p.urlVi].filter(Boolean)) expect(new URL(u!).protocol, p.id).toBe('https:')
      expect(partnerLogo(p), p.id).toBeTruthy()
      expect(en.partners.desc[p.id], p.id).toBeTruthy()
      expect(viCatalog.partners.desc[p.id], p.id).toBeTruthy()
    }
  })

  it('give every proxy a known type and every tool a known section, both named in both languages', () => {
    for (const p of partnersOf('proxy')) {
      expect(p.types?.length, p.id).toBeGreaterThan(0)
      for (const k of p.types!) expect(proxyTypes, p.id).toContain(k)
    }
    for (const p of partnersOf('tool')) expect(toolGroups, p.id).toContain(p.group)
    for (const k of proxyTypes) expect([en.partners.types[k], viCatalog.partners.types[k]].every(Boolean), k).toBe(true)
    for (const g of toolGroups) expect([en.partners.groups[g], viCatalog.partners.groups[g]].every(Boolean), g).toBe(true)
  })

  it('keeps the quality-first recommendation order, including specialist and trial providers', () => {
    expect(partnersOf('proxy').map((p) => p.id)).toEqual([
      'oxylabs', 'decodo', 'brightdata', 'soax', 'evomi', 'proxyempire', 'nodemaven',
      'webshare', 'iproyal', 'proxyseller', 'massive', 'dataimpulse', 'rayobyte', 'infatica',
      'marsproxies', 'proxycheap',
      'froxy', 'coronium', 'ltesocks', '9proxy', 'lumiproxy', 'rapidproxy',
    ])
  })

  it('includes every Craft app from the suite', () => {
    const ids = partnersOf('tool').map((p) => p.id)
    for (const id of ['photocraft', 'vectorcraft', 'filmcraft', 'lightcraft', 'pdfcraft', 'effectcraft', 'designcraft', 'figcraft', 'wordcraft', 'soundcraft', 'deckcraft', 'cadcraft', 'gridcraft', 'texturecraft', 'craft-anything', 'solidcraft']) {
      expect(ids, id).toContain(id)
    }
  })

  it('list PostClaw first among the tools and nowhere among proxies', () => {
    expect(partnersOf('tool')[0].id).toBe('postclaw')
    expect(partnersOf('proxy').map((p) => p.id)).not.toContain('postclaw')
  })

  it('send PostClaw to postclaw.fun with UTM tags, in the app language', () => {
    const pc = partners.find((p) => p.id === 'postclaw')!
    const en = new URL(partnerLink(pc, 'en'))
    expect(en.host).toBe('postclaw.fun')
    expect(en.pathname).toBe('/')
    expect(Object.fromEntries(en.searchParams)).toEqual({ utm_source: 'anyprofile', utm_medium: 'app', utm_campaign: 'creator-tools' })
    const vi = new URL(partnerLink(pc, 'vi'))
    expect(vi.pathname).toBe('/vi/')
    expect(vi.searchParams.get('utm_source')).toBe('anyprofile')
  })

  it.each([
    ['marsproxies', 'https://marsproxies.sjv.io/bkJz7g'],
    ['proxycheap', 'https://app.proxy-cheap.com/r/NZNb2Ik0'],
    ['brightdata', 'https://get.brightdata.com/nhyeetzbk27n'],
    ['webshare', 'https://www.webshare.io/?referral_code=e18t5gbij3d3'],
    ['rapidproxy', 'https://www.rapidproxy.io/?code=GN72DW2EP'],
    ['dataimpulse', 'https://dataimpulse.com/?aff=e68ede41-33fe-4532-ab36-2240e6a1142d'],
    ['rayobyte', 'https://rayobyte.com/products/residential-proxies/?ref=chau-d11bd8'],
    ['lumiproxy', 'https://www.lumiproxy.com/?invite_code=97EJLV'],
    ['decodo', 'https://dashboard.decodo.com/register?referral_code=2973fec4bd68c176365c5cf1ef80bade6eaccdde'],
  ])('uses the %s affiliate URL in both app languages', (id, url) => {
    const partner = partners.find((p) => p.id === id)!
    for (const lang of ['en', 'vi']) {
      expect(partnerLink(partner, lang)).toBe(url)
    }
  })

  it('uses the Oxylabs affiliate URL in both app languages and marks it recommended', () => {
    const oxylabs = partnersOf('proxy').find((p) => p.id === 'oxylabs')!
    expect(oxylabs.recommended).toBe(true)
    for (const lang of ['en', 'vi']) {
      expect(partnerLink(oxylabs, lang)).toBe('https://oxylabs.go2cloud.org/aff_c?offer_id=7&aff_id=2794')
    }
  })

  it('leave provider links as they are', () => {
    for (const p of partnersOf('proxy')) expect(partnerLink(p, 'vi')).toBe(p.url)
  })
})
