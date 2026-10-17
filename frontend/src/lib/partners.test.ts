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

  it('leave provider links as they are', () => {
    for (const p of partnersOf('proxy')) expect(partnerLink(p, 'vi')).toBe(p.url)
  })
})
