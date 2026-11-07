// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { t } from '../i18n'
import { tagColor } from '../lib/tags'
import { modLabel } from '../lib/platform'
import type { Site } from '../lib/sites'
import type { service } from '../api'
import { SiteGlyph } from './site-glyph'
import { Icon, LogoMark, type IconName } from './icons'
import { BuyProxiesButton } from './partner-grid'

export type Page = 'profiles' | 'mobile' | 'proxies' | 'trash' | 'settings'
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
  mobileCount: number
  engineLabel: string
  engineReady: boolean
  pro?: service.ProStatus | null
  onNavigate: (page: Page, view?: View) => void
  onNewProfile: () => void
  onOpenTools: () => void
}

export function Sidebar({ page, view, counts, tags, sites, siteCounts, proxyCount, mobileCount, engineLabel, engineReady, pro, onNavigate, onNewProfile, onOpenTools }: Props) {
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
        {pro?.active && (
          <span className="brand-tier" aria-label="Pro">
            <svg className="brand-tier-star" viewBox="0 0 16 16" aria-hidden="true">
              <defs>
                <linearGradient id="pro-star-grad" x1="0%" y1="0%" x2="100%" y2="100%">
                  <stop offset="0%" stopColor="#4f8bff" />
                  <stop offset="52%" stopColor="#e07b8b" />
                  <stop offset="100%" stopColor="#e8b455" />
                </linearGradient>
              </defs>
              <path fill="url(#pro-star-grad)" d="M8 1.2c.35 3.1 1.6 4.95 2.75 5.45.55.25 1.75.65 4.05 1.35-2.3.7-3.5 1.1-4.05 1.35C9.6 9.85 8.35 11.7 8 14.8c-.35-3.1-1.6-4.95-2.75-5.45-.55-.25-1.75-.65-4.05-1.35 2.3-.7 3.5-1.1 4.05-1.35C6.4 6.15 7.65 4.3 8 1.2z" />
            </svg>
            <span className="brand-tier-text">PRO</span>
          </span>
        )}
      </div>
      <button className="btn primary block newbtn nodrag" onClick={onNewProfile} title={`${t.nav.newProfile} ${modLabel}N`}>
        <Icon name="plus" />
        {t.nav.newProfile}
      </button>
      <nav className="sidenav" aria-label={t.appName}>
        {item(t.nav.all, 'profiles', onProfiles('all'), () => onNavigate('profiles', 'all'), counts.all)}
        {item(t.nav.mobile, 'mobile', page === 'mobile', () => onNavigate('mobile'), mobileCount)}
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
        <BuyProxiesButton className="navitem nodrag" />
        {item(t.nav.tools, 'spark', false, onOpenTools)}
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
