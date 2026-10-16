// SPDX-License-Identifier: AGPL-3.0-or-later
import { describe, expect, it } from 'vitest'
import { en } from '../i18n/en'
import { vi as viCatalog } from '../i18n/vi'
import { partnerLink, partners, partnersOf } from './partners'

describe('partners', () => {
  it('have unique ids, https links and a description in both languages', () => {
    expect(new Set(partners.map((p) => p.id)).size).toBe(partners.length)
    for (const p of partners) {
      for (const u of [p.url, p.urlVi].filter(Boolean)) expect(new URL(u!).protocol, p.id).toBe('https:')
      expect(en.partners.desc[p.id], p.id).toBeTruthy()
      expect(viCatalog.partners.desc[p.id], p.id).toBeTruthy()
    }
  })

  it('list proxies under proxy and PostClaw under tools', () => {
    expect(partnersOf('proxy').length).toBeGreaterThan(0)
    expect(partnersOf('tool').map((p) => p.id)).toEqual(['postclaw'])
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
