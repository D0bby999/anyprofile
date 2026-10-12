// SPDX-License-Identifier: AGPL-3.0-or-later
import type { Site } from '../lib/sites'
import { glyphColors } from '../lib/sites'
import { siteIconPaths } from '../sites/icons.gen'

/** A platform badge: the brand mark on its color, or the first letter. */
export function SiteGlyph({ site }: { site: Site }) {
  const path = site.icon ? siteIconPaths[site.icon] : undefined
  return (
    <span className="glyph" style={glyphColors(site.color)} aria-hidden="true">
      {path ? (
        <svg viewBox="0 0 24 24"><path fill="currentColor" d={path} /></svg>
      ) : (
        <b className="glyphletter">{site.name.trim().charAt(0).toUpperCase() || '?'}</b>
      )}
    </span>
  )
}
