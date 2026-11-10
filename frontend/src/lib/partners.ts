// SPDX-License-Identifier: AGPL-3.0-or-later
// Proxy providers (Buy proxies) and creator tools (the Creator tools page). The list and the
// logos ship with the app: fetching either would be a request that docs/privacy-and-terms.md
// does not allow. Links open in the default browser, never in a profile.

export type PartnerKind = 'proxy' | 'tool'
export type ProxyType = 'residential' | 'isp' | 'mobile' | 'datacenter'
export type ToolGroup = 'publish' | 'video' | 'record' | 'audio' | 'image' | 'craft'

export type Partner = {
  id: string
  name: string
  kind: PartnerKind
  url: string
  /** The partner's Vietnamese page, when it has one. */
  urlVi?: string
  /** Our own products: their links carry UTM tags so the visits can be counted there. */
  own?: boolean
  /** Highlighted editorial recommendation at the top of the list. */
  recommended?: boolean
  /** Proxy providers: what they sell. */
  types?: ProxyType[]
  /** Creator tools: the section they sit in. */
  group?: ToolGroup
  openSource?: boolean
}

export const proxyTypes: ProxyType[] = ['residential', 'isp', 'mobile', 'datacenter']
export const toolGroups: ToolGroup[] = ['publish', 'video', 'record', 'audio', 'image', 'craft']

// Quality-first recommendation order, reviewed 2026-11-05: public infrastructure tests,
// reliability, coverage and support before price; less-evidenced options and specialists follow.
// This is an editorial recommendation, not our own benchmark or an affiliate payout ranking.
// SOAX, Massive and Froxy link to their home pages until their referral links arrive.
const all = ['residential', 'isp', 'mobile', 'datacenter'] satisfies ProxyType[]
export const partners: Partner[] = [
  { id: 'oxylabs', name: 'Oxylabs', kind: 'proxy', url: 'https://oxylabs.go2cloud.org/aff_c?offer_id=7&aff_id=2794', recommended: true, types: all },
  { id: 'decodo', name: 'Decodo', kind: 'proxy', url: 'https://dashboard.decodo.com/register?referral_code=2973fec4bd68c176365c5cf1ef80bade6eaccdde', types: all },
  { id: 'brightdata', name: 'Bright Data', kind: 'proxy', url: 'https://get.brightdata.com/nhyeetzbk27n', types: all },
  { id: 'soax', name: 'SOAX', kind: 'proxy', url: 'https://soax.com/', types: all },
  { id: 'evomi', name: 'Evomi', kind: 'proxy', url: 'https://evomi.com/?p=5Z7DMFN3ME', types: all },
  { id: 'proxyempire', name: 'ProxyEmpire', kind: 'proxy', url: 'https://proxyempire.io/?ref=oddhngm', types: all },
  { id: 'nodemaven', name: 'NodeMaven', kind: 'proxy', url: 'https://nodemaven.com/?a_aid=anyprofile', types: ['residential', 'isp', 'mobile'] },
  { id: 'webshare', name: 'Webshare', kind: 'proxy', url: 'https://www.webshare.io/?referral_code=e18t5gbij3d3', types: ['residential', 'isp', 'datacenter'] },
  { id: 'iproyal', name: 'IPRoyal', kind: 'proxy', url: 'https://iproyal.sjv.io/X4aYqG', types: all },
  { id: 'proxyseller', name: 'Proxy-Seller', kind: 'proxy', url: 'https://proxy-seller.com/?partner=JUJ9ZRI3NSJI1N', types: all },
  { id: 'massive', name: 'Massive', kind: 'proxy', url: 'https://www.joinmassive.com/', types: ['residential', 'isp'] },
  { id: 'dataimpulse', name: 'DataImpulse', kind: 'proxy', url: 'https://dataimpulse.com/?aff=e68ede41-33fe-4532-ab36-2240e6a1142d', types: ['residential', 'mobile', 'datacenter'] },
  { id: 'rayobyte', name: 'Rayobyte', kind: 'proxy', url: 'https://rayobyte.com/products/residential-proxies/?ref=chau-d11bd8', types: all },
  { id: 'infatica', name: 'Infatica', kind: 'proxy', url: 'https://dashboard.infatica.io/aff.php?aff=902', types: ['residential', 'isp', 'datacenter'] },
  { id: 'marsproxies', name: 'MarsProxies', kind: 'proxy', url: 'https://marsproxies.sjv.io/bkJz7g', types: all },
  { id: 'proxycheap', name: 'Proxy-Cheap', kind: 'proxy', url: 'https://app.proxy-cheap.com/r/NZNb2Ik0', types: all },
  { id: 'froxy', name: 'Froxy', kind: 'proxy', url: 'https://froxy.com/', types: ['residential', 'mobile', 'datacenter'] },
  { id: 'coronium', name: 'Coronium', kind: 'proxy', url: 'https://dashboard.coronium.io/en/sign-up#765b230d864e88ca7c0d800d4fd17c3b', types: ['mobile'] },
  { id: 'ltesocks', name: 'LTESocks', kind: 'proxy', url: 'https://my.ltesocks.io/register?referrerToken=KSFSK9JQHEFK0KFL&referrer=01a17d5a-f6f2-7125-a17a-9dd8e2d583e3', types: ['mobile'] },
  { id: '9proxy', name: '9Proxy', kind: 'proxy', url: 'https://9proxy.com/sign-up?inviteCode=9P_mXH5nLuQ', types: ['residential'] },
  { id: 'lumiproxy', name: 'LumiProxy', kind: 'proxy', url: 'https://www.lumiproxy.com/?invite_code=97EJLV', types: ['residential', 'isp', 'datacenter'] },
  { id: 'rapidproxy', name: 'Rapidproxy', kind: 'proxy', url: 'https://www.rapidproxy.io/?code=GN72DW2EP', types: ['residential', 'isp'] },

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
  { id: 'photocraft', name: 'PhotoCraft', kind: 'tool', url: 'https://getartcraft.com/apps/photocraft', group: 'craft', openSource: true },
  { id: 'vectorcraft', name: 'VectorCraft', kind: 'tool', url: 'https://getartcraft.com/apps/vectorcraft', group: 'craft', openSource: true },
  { id: 'lightcraft', name: 'LightCraft', kind: 'tool', url: 'https://getartcraft.com/apps/lightcraft', group: 'craft', openSource: true },
  { id: 'pdfcraft', name: 'PdfCraft', kind: 'tool', url: 'https://getartcraft.com/apps/pdfcraft', group: 'craft', openSource: true },
  { id: 'effectcraft', name: 'EffectCraft', kind: 'tool', url: 'https://getartcraft.com/apps/effectcraft', group: 'craft', openSource: true },
  { id: 'designcraft', name: 'DesignCraft', kind: 'tool', url: 'https://getartcraft.com/apps/designcraft', group: 'craft', openSource: true },
  { id: 'figcraft', name: 'FigCraft', kind: 'tool', url: 'https://getartcraft.com/apps/figcraft', group: 'craft', openSource: true },
  { id: 'wordcraft', name: 'WordCraft', kind: 'tool', url: 'https://getartcraft.com/apps/wordcraft', group: 'craft', openSource: true },
  { id: 'soundcraft', name: 'SoundCraft', kind: 'tool', url: 'https://getartcraft.com/apps/soundcraft', group: 'craft', openSource: true },
  { id: 'deckcraft', name: 'DeckCraft', kind: 'tool', url: 'https://getartcraft.com/apps/deckcraft', group: 'craft', openSource: true },
  { id: 'cadcraft', name: 'CADCraft', kind: 'tool', url: 'https://getartcraft.com/apps/cadcraft', group: 'craft', openSource: true },
  { id: 'gridcraft', name: 'GridCraft', kind: 'tool', url: 'https://getartcraft.com/apps/gridcraft', group: 'craft', openSource: true },
  { id: 'texturecraft', name: 'TextureCraft', kind: 'tool', url: 'https://getartcraft.com/apps/texturecraft', group: 'craft', openSource: true },
  { id: 'craft-anything', name: 'Craft Anything', kind: 'tool', url: 'https://github.com/storytold/craft-anything', group: 'craft', openSource: true },
  { id: 'solidcraft', name: 'SolidCraft', kind: 'tool', url: 'https://github.com/tangency-labs/solidcraft', group: 'craft', openSource: true },
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
