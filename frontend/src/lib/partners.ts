// SPDX-License-Identifier: AGPL-3.0-or-later
// Proxy providers (Buy proxies) and creator tools (the Creator tools page). The list and the
// logos ship with the app: fetching either would be a request that docs/privacy-and-terms.md
// does not allow. Links open in the default browser, never in a profile.

export type PartnerKind = 'proxy' | 'tool'
export type ProxyType = 'residential' | 'isp' | 'mobile' | 'datacenter'
export type ToolGroup = 'publish' | 'video' | 'record' | 'audio' | 'image'

export type Partner = {
  id: string
  name: string
  kind: PartnerKind
  url: string
  /** The partner's Vietnamese page, when it has one. */
  urlVi?: string
  /** Our own products: their links carry UTM tags so the visits can be counted there. */
  own?: boolean
  /** Proxy providers: what they sell. */
  types?: ProxyType[]
  /** Creator tools: the section they sit in. */
  group?: ToolGroup
  openSource?: boolean
}

export const proxyTypes: ProxyType[] = ['residential', 'isp', 'mobile', 'datacenter']
export const toolGroups: ToolGroup[] = ['publish', 'video', 'record', 'audio', 'image']

// Providers are the ones on hidemyacc.com/partners that suit long-lived accounts: established,
// with static ISP, sticky residential or real mobile IPs. Links are our affiliate referral
// links where an account exists; SOAX, Massive and Froxy still link to their home pages
// until theirs arrive. Best fit first.
const all = ['residential', 'isp', 'mobile', 'datacenter'] satisfies ProxyType[]
export const partners: Partner[] = [
  { id: 'iproyal', name: 'IPRoyal', kind: 'proxy', url: 'https://iproyal.com/?r=1734020', types: all },
  { id: 'nodemaven', name: 'NodeMaven', kind: 'proxy', url: 'https://nodemaven.com/?a_aid=anyprofile', types: ['residential', 'isp', 'mobile'] },
  { id: 'decodo', name: 'Decodo', kind: 'proxy', url: 'https://dashboard.decodo.com/register?referral_code=2973fec4bd68c176365c5cf1ef80bade6eaccdde', types: all },
  { id: 'soax', name: 'SOAX', kind: 'proxy', url: 'https://soax.com/', types: all },
  { id: 'proxyseller', name: 'Proxy-Seller', kind: 'proxy', url: 'https://proxy-seller.com/?partner=JUJ9ZRI3NSJI1N', types: all },
  { id: 'marsproxies', name: 'MarsProxies', kind: 'proxy', url: 'https://marsproxies.sjv.io/bkJz7g', types: all },
  { id: 'evomi', name: 'Evomi', kind: 'proxy', url: 'https://evomi.com/?p=5Z7DMFN3ME', types: all },
  { id: 'proxyempire', name: 'ProxyEmpire', kind: 'proxy', url: 'https://proxyempire.io/?ref=oddhngm', types: all },
  { id: 'infatica', name: 'Infatica', kind: 'proxy', url: 'https://dashboard.infatica.io/aff.php?aff=902', types: ['residential', 'isp', 'datacenter'] },
  { id: 'massive', name: 'Massive', kind: 'proxy', url: 'https://www.joinmassive.com/', types: ['residential', 'isp'] },
  { id: 'froxy', name: 'Froxy', kind: 'proxy', url: 'https://froxy.com/', types: ['residential', 'mobile', 'datacenter'] },
  { id: '9proxy', name: '9Proxy', kind: 'proxy', url: 'https://9proxy.com/sign-up?inviteCode=9P_mXH5nLuQ', types: ['residential'] },
  { id: 'coronium', name: 'Coronium', kind: 'proxy', url: 'https://dashboard.coronium.io/en/sign-up#765b230d864e88ca7c0d800d4fd17c3b', types: ['mobile'] },
  { id: 'ltesocks', name: 'LTESocks', kind: 'proxy', url: 'https://my.ltesocks.io/register?referrerToken=KSFSK9JQHEFK0KFL&referrer=01a17d5a-f6f2-7125-a17a-9dd8e2d583e3', types: ['mobile'] },

  // postclaw.io is another company with the same name.
  { id: 'postclaw', name: 'PostClaw', kind: 'tool', url: 'https://postclaw.fun/', urlVi: 'https://postclaw.fun/vi/', own: true, group: 'publish' },
  // Free and open source, maintained, and on macOS and Windows.
  { id: 'filmcraft', name: 'FilmCraft', kind: 'tool', url: 'https://getartcraft.com/apps/filmcraft', group: 'video', openSource: true },
  { id: 'shotcut', name: 'Shotcut', kind: 'tool', url: 'https://www.shotcut.org/', group: 'video', openSource: true },
  { id: 'losslesscut', name: 'LosslessCut', kind: 'tool', url: 'https://losslesscut.app/', group: 'video', openSource: true },
  { id: 'handbrake', name: 'HandBrake', kind: 'tool', url: 'https://handbrake.fr/', group: 'video', openSource: true },
  { id: 'obs', name: 'OBS Studio', kind: 'tool', url: 'https://obsproject.com/', group: 'record', openSource: true },
  { id: 'cap', name: 'Cap', kind: 'tool', url: 'https://cap.so/', group: 'record', openSource: true },
  { id: 'buzz', name: 'Buzz', kind: 'tool', url: 'https://chidiwilliams.github.io/buzz/', group: 'audio', openSource: true },
  { id: 'audacity', name: 'Audacity', kind: 'tool', url: 'https://www.audacityteam.org/', group: 'audio', openSource: true },
  { id: 'upscayl', name: 'Upscayl', kind: 'tool', url: 'https://upscayl.org/', group: 'image', openSource: true },
]

export const partnersOf = (kind: PartnerKind) => partners.filter((p) => p.kind === kind)

const logos = import.meta.glob<string>('../assets/partners/*.{png,svg}', { eager: true, import: 'default' })

/** The partner's logo, bundled under assets/partners/<id>.png or .svg. */
export function partnerLogo(p: Partner): string | undefined {
  return logos[`../assets/partners/${p.id}.svg`] ?? logos[`../assets/partners/${p.id}.png`]
}

export function partnerLink(p: Partner, lang: string): string {
  const u = new URL(lang === 'vi' && p.urlVi ? p.urlVi : p.url)
  if (p.own) {
    u.searchParams.set('utm_source', 'anyprofile')
    u.searchParams.set('utm_medium', 'app')
    u.searchParams.set('utm_campaign', p.kind === 'tool' ? 'creator-tools' : 'buy-proxies')
  }
  return u.toString()
}
