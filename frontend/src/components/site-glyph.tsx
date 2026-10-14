// SPDX-License-Identifier: AGPL-3.0-or-later
import type { Site } from '../lib/sites'
import { glyphColors } from '../lib/sites'
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
