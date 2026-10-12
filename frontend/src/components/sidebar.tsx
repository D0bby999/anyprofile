// SPDX-License-Identifier: AGPL-3.0-or-later
import { t } from '../i18n'
import { ChannelGlyph, Icon, LogoMark, type IconName } from './icons'

export type Page = 'profiles' | 'proxies' | 'trash' | 'settings'
/** What the profile table shows: everything, the running ones, or one channel. */
export type View = 'all' | 'running' | 'tiktok' | 'youtube' | 'other'

type Props = {
  page: Page
  view: View
  counts: Record<View, number>
  proxyCount: number
  engineLabel: string
  engineReady: boolean
  onNavigate: (page: Page, view?: View) => void
  onNewProfile: () => void
}

export function Sidebar({ page, view, counts, proxyCount, engineLabel, engineReady, onNavigate, onNewProfile }: Props) {
  const item = (label: string, icon: IconName, active: boolean, onClick: () => void, count?: number, live = false) => (
    <button className="navitem nodrag" aria-current={active ? 'page' : undefined} onClick={onClick}>
      <Icon name={icon} />
      <span>{label}</span>
      {count !== undefined && <span className={`cnt${live && count > 0 ? ' live' : ''}`}>{count}</span>}
    </button>
  )
  const onProfiles = (v: View) => page === 'profiles' && view === v

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

        <div className="navsection">{t.nav.channels}</div>
        {(['tiktok', 'youtube', 'other'] as const).map((k) => (
          <button key={k} className="navitem nodrag" aria-current={onProfiles(k) ? 'page' : undefined}
            onClick={() => onNavigate('profiles', k)}>
            <ChannelGlyph kind={k} />
            <span>{t.channel[k]}</span>
            <span className="cnt">{counts[k]}</span>
          </button>
        ))}
      </nav>
      <div className="sidefoot">
        {item(t.nav.settings, 'settings', page === 'settings', () => onNavigate('settings'))}
        <div className="enginechip">
          <span className={`state${engineReady ? ' ok' : ''}`} />
          {engineLabel}
        </div>
      </div>
    </aside>
  )
}
