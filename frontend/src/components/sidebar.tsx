// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { t } from '../i18n'
import { tagColor } from '../lib/tags'
import { modLabel } from '../lib/platform'
import type { Site } from '../lib/sites'
import { SiteGlyph } from './site-glyph'
import { Icon, LogoMark, type IconName } from './icons'

export type Page = 'profiles' | 'proxies' | 'trash' | 'tools' | 'settings'
/** What the profile table shows: everything, the running ones, the ones that
 *  need attention (both picked from the status bar), one platform, or one tag. */
export type View = 'all' | 'running' | 'attention' | `site:${string}` | `tag:${string}`

const TAGS_SHOWN = 12

type Props = {
  page: Page
  view: View
  counts: { all: number }
  /** Tags with profile counts, most used first. */
  tags: [string, number][]
  sites: Site[]
  siteCounts: Map<string, number>
  proxyCount: number
  engineLabel: string
  engineReady: boolean
  onNavigate: (page: Page, view?: View) => void
  onNewProfile: () => void
}

export function Sidebar({ page, view, counts, tags, sites, siteCounts, proxyCount, engineLabel, engineReady, onNavigate, onNewProfile }: Props) {
  const [tagsOpen, setTagsOpen] = useState(true)
  const [allTags, setAllTags] = useState(false)
  const item = (label: string, icon: IconName, active: boolean, onClick: () => void, count?: number, tone: '' | 'live' | 'warn' = '') => (
    <button className="navitem nodrag" aria-current={active ? 'page' : undefined} onClick={onClick}>
      <Icon name={icon} />
      <span>{label}</span>
      {count !== undefined && <span className={`cnt${tone && count > 0 ? ` ${tone}` : ''}`}>{count}</span>}
    </button>
  )
  const shownTags = allTags ? tags : tags.slice(0, TAGS_SHOWN)
  const onProfiles = (v: View) => page === 'profiles' && view === v

  const usedSites = sites
    .filter((s) => (siteCounts.get(s.id) ?? 0) > 0)
    .sort((a, b) => (siteCounts.get(b.id)! - siteCounts.get(a.id)!) || a.name.localeCompare(b.name))

  return (
    <aside className="sidebar drag">
      <div className="brandrow">
        <LogoMark />
        <b>{t.appName}</b>
      </div>
      <button className="btn primary block newbtn nodrag" onClick={onNewProfile} title={`${t.nav.newProfile} ${modLabel}N`}>
        <Icon name="plus" />
        {t.nav.newProfile}
      </button>
      <nav className="sidenav" aria-label={t.appName}>
        {item(t.nav.all, 'profiles', onProfiles('all'), () => onNavigate('profiles', 'all'), counts.all)}
        {item(t.nav.proxies, 'proxy', page === 'proxies', () => onNavigate('proxies'), proxyCount)}
        {item(t.nav.trash, 'trash', page === 'trash', () => onNavigate('trash'))}

        {usedSites.length > 0 && (
          <>
            <div className="navsection">{t.nav.sites}</div>
            {usedSites.map((s) => (
              <button key={s.id} className="navitem nodrag" aria-current={onProfiles(`site:${s.id}`) ? 'page' : undefined}
                onClick={() => onNavigate('profiles', `site:${s.id}`)}>
                <SiteGlyph site={s} />
                <span>{s.name}</span>
                <span className="cnt">{siteCounts.get(s.id)}</span>
              </button>
            ))}
          </>
        )}

        {tags.length > 0 && (
          <>
            <button type="button" className="navsection navtoggle nodrag" aria-expanded={tagsOpen} onClick={() => setTagsOpen(!tagsOpen)}>
              <Icon name="chevron" className="icon xs" />{t.nav.tags}
            </button>
            {tagsOpen && shownTags.map(([tag, n]) => (
              <button key={tag} className="navitem nodrag" aria-current={onProfiles(`tag:${tag}`) ? 'page' : undefined}
                onClick={() => onNavigate('profiles', `tag:${tag}`)}>
                <span className="tagdot" style={{ background: `var(--tag-${tagColor(tag)})` }} />
                <span className="navlabel">{tag}</span>
                <span className="cnt">{n}</span>
              </button>
            ))}
            {tagsOpen && tags.length > TAGS_SHOWN && (
              <button type="button" className="navmore nodrag" onClick={() => setAllTags(!allTags)}>
                {allTags ? t.nav.showFewer : t.nav.showAll(tags.length)}
              </button>
            )}
          </>
        )}
      </nav>
      <div className="sidefoot">
        {item(t.nav.tools, 'spark', page === 'tools', () => onNavigate('tools'))}
        {item(t.nav.settings, 'settings', page === 'settings', () => onNavigate('settings'))}
        {engineLabel && (engineReady ? (
          <div className="enginechip"><span className="state ok" />{engineLabel}</div>
        ) : (
          <button type="button" className="enginechip nodrag" onClick={() => onNavigate('settings')}>
            <span className="state warn" />{engineLabel}
          </button>
        ))}
      </div>
    </aside>
  )
}
