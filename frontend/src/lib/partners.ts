// SPDX-License-Identifier: AGPL-3.0-or-later
// Proxy providers (Buy proxies) and creator tools (the Creator tools page). The list
// ships with the app: fetching it would be a request that docs/privacy-and-terms.md
// does not allow. Links open in the default browser, never in a profile.

export type PartnerKind = 'proxy' | 'tool'

export type Partner = {
  id: string
  name: string
  kind: PartnerKind
  url: string
  /** The partner's Vietnamese page, when it has one. */
  urlVi?: string
  /** Our own products: their links carry UTM tags so the visits can be counted there. */
  own?: boolean
}

// Proxy links are the providers' home pages until the affiliate accounts exist; swap in
// each referral link as it arrives. Best fit for long-lived accounts first.
export const partners: Partner[] = [
  { id: 'iproyal', name: 'IPRoyal', kind: 'proxy', url: 'https://iproyal.com/' },
  { id: 'decodo', name: 'Decodo', kind: 'proxy', url: 'https://decodo.com/' },
  { id: 'proxycheap', name: 'Proxy-Cheap', kind: 'proxy', url: 'https://www.proxy-cheap.com/' },
  { id: 'soax', name: 'SOAX', kind: 'proxy', url: 'https://soax.com/' },
  { id: 'nodemaven', name: 'NodeMaven', kind: 'proxy', url: 'https://nodemaven.com/' },
  { id: 'webshare', name: 'Webshare', kind: 'proxy', url: 'https://www.webshare.io/' },
  // postclaw.io is another company with the same name.
  { id: 'postclaw', name: 'PostClaw', kind: 'tool', url: 'https://postclaw.fun/', urlVi: 'https://postclaw.fun/vi/', own: true },
]

export const partnersOf = (kind: PartnerKind) => partners.filter((p) => p.kind === kind)

export function partnerLink(p: Partner, lang: string): string {
  const u = new URL(lang === 'vi' && p.urlVi ? p.urlVi : p.url)
  if (p.own) {
    u.searchParams.set('utm_source', 'anyprofile')
    u.searchParams.set('utm_medium', 'app')
    u.searchParams.set('utm_campaign', p.kind === 'tool' ? 'creator-tools' : 'buy-proxies')
  }
  return u.toString()
}
