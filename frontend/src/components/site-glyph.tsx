// SPDX-License-Identifier: AGPL-3.0-or-later
import type { Site } from '../lib/sites'
import { glyphColors, siteById } from '../lib/sites'
import { siteIcons } from '../sites/icons.gen'

/** A platform badge: the brand mark on its color, or the first letter. */
export function SiteGlyph({ site }: { site: Site }) {
  const icon = site.icon ? siteIcons[site.icon] : undefined
  return (
    <span className="glyph" style={glyphColors(site.color)} aria-hidden="true">
      {icon ? (
        <svg viewBox={icon.viewBox}><path fill="currentColor" d={icon.d} /></svg>
      ) : (
        <b className="glyphletter">{site.name.trim().charAt(0).toUpperCase() || '?'}</b>
      )}
    </span>
  )
}

/** A profile's platforms: every badge, the main platform's name, and how many
 *  more there are. The full list is in the tooltip. */
/** At most this many icons in a row; the label and the tooltip count the rest. */
const MAX_GLYPHS = 3

export function SiteBadges({ ids, sites }: { ids: string[]; sites: Site[] }) {
  const list = (ids.length ? ids : ['other']).map((id) => siteById(sites, id))
  const names = list.map((s) => s.name).join(', ')
  return (
    <span className="platform" title={names}>
      <span className="glyphs">{list.slice(0, MAX_GLYPHS).map((s) => <SiteGlyph key={s.id} site={s} />)}</span>
      <span className="plabel">{list[0].name}{list.length > 1 ? ` +${list.length - 1}` : ''}</span>
    </span>
  )
}
