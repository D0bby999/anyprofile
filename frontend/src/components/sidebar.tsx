// SPDX-License-Identifier: AGPL-3.0-or-later
import { t } from '../i18n'
import type { Site } from '../lib/sites'
import { SiteGlyph } from './site-glyph'
import { Icon, LogoMark, type IconName } from './icons'

export type Page = 'profiles' | 'proxies' | 'trash' | 'settings'
/** What the profile table shows: everything, the running ones, or one platform. */
export type View = 'all' | 'running' | `site:${string}`

type Props = {
  page: Page
  view: View
  counts: { all: number; running: number }
  sites: Site[]
  siteCounts: Map<string, number>
  proxyCount: number
  engineLabel: string
  engineReady: boolean
  onNavigate: (page: Page, view?: View) => void
  onNewProfile: () => void
}

export function Sidebar({ page, view, counts, sites, siteCounts, proxyCount, engineLabel, engineReady, onNavigate, onNewProfile }: Props) {
  const item = (label: string, icon: IconName, active: boolean, onClick: () => void, count?: number, live = false) => (
    <button className="navitem nodrag" aria-current={active ? 'page' : undefined} onClick={onClick}>
      <Icon name={icon} />
      <span>{label}</span>
      {count !== undefined && <span className={`cnt${live && count > 0 ? ' live' : ''}`}>{count}</span>}
    </button>
  )
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
      <button className="btn primary block newbtn nodrag" onClick={onNewProfile}>
        <Icon name="plus" />
        {t.nav.newProfile}
      </button>
      <nav className="sidenav" aria-label={t.appName}>
        {item(t.nav.all, 'profiles', onProfiles('all'), () => onNavigate('profiles', 'all'), counts.all)}
        {item(t.nav.running, 'running', onProfiles('running'), () => onNavigate('profiles', 'running'), counts.running, true)}
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
      </nav>
      <div className="sidefoot">
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
