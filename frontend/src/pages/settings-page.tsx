// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useRef, useState, type ReactNode } from 'react'
import { main, service } from '../api'
import { t, type Language } from '../i18n'
import type { ThemeChoice } from '../lib/theme'
import { useSites } from '../hooks/use-sites'
import { Icon, type IconName } from '../components/icons'
import { SiteSettings } from '../components/site-settings'
import { StorageSettings } from '../components/storage-settings'
import { BackupSettings } from '../components/backup-settings'
import { DefaultsSettings, OnOpenSettings } from '../components/defaults-settings'
import { UpdateSettings } from '../components/update-settings'
import { ProSettings } from '../components/pro-settings'
import { SupportSettings } from '../components/support-settings'
import { DonateSettings } from '../components/donate-settings'
import { shownDonateLinks } from '../lib/donate'
import { APISettings } from '../components/api-settings'
import { ExtensionSettings } from '../components/extension-settings'
import { PageHead } from '../components/page-head'
import { EngineSettings } from '../components/engine-settings'
import { scrollSettingsTo } from '../lib/scroll'

/** Settings come in groups, most used first; the technical ones sit near the end. */
const groupKeys = ['general', 'profiles', 'automation', 'data', 'engines', 'about'] as const
type GroupKey = typeof groupKeys[number]


type Props = {
  engines: main.EngineInfo[]
  engineCounts: Record<string, number>
  /** Browsers open now; an update waits until they are closed. */
  openProfiles: number
  defaults: service.Defaults
  onEnginesChanged: () => void
  theme: ThemeChoice
  onTheme: (c: ThemeChoice) => void
  fileVaultOn: boolean
  lang: Language
  onLanguage: (l: Language) => void
  /** Bumped by the menu bar's Check for Updates. */
  checkUpdate?: number
}

export function SettingsPage({ engines, engineCounts, defaults, onEnginesChanged, theme, onTheme, fileVaultOn, lang, onLanguage, openProfiles, checkUpdate }: Props) {
  const { sites, reload: reloadSites } = useSites()
  const themes: [ThemeChoice, string, IconName][] = [['system', t.settings.themeSystem, 'monitor'], ['light', t.settings.themeLight, 'sun'], ['dark', t.settings.themeDark, 'moon']]

  // The group in view is lit in the side list; the list jumps to a group.
  const scroller = useRef<HTMLDivElement>(null)
  const [current, setCurrent] = useState<GroupKey>('general')
  // A group picked in the list stays lit even when it is too short to reach the top.
  const jumpedAt = useRef(0)
  useEffect(() => {
    const root = scroller.current
    if (!root || typeof IntersectionObserver === 'undefined') return
    const seen = new Map<string, boolean>()
    const io = new IntersectionObserver((entries) => {
      for (const e of entries) seen.set(e.target.id, e.isIntersecting)
      if (Date.now() - jumpedAt.current < 600) return
      const first = groupKeys.find((k) => seen.get(`set-${k}`))
      if (first) setCurrent(first)
    }, { root, rootMargin: '0px 0px -60% 0px' })
    for (const k of groupKeys) { const el = document.getElementById(`set-${k}`); if (el) io.observe(el) }
    return () => io.disconnect()
  }, [])
  const jump = (k: GroupKey) => { jumpedAt.current = Date.now(); setCurrent(k); scrollSettingsTo(`set-${k}`) }

  const group = (k: GroupKey, children: ReactNode) => (
    <section className="setgroup" id={`set-${k}`} aria-labelledby={`set-${k}-title`}>
      <h2 className="setgroup-title" id={`set-${k}-title`}>{t.settings.groups[k]}</h2>
      {children}
    </section>
  )

  return (
    <>
      <PageHead title={t.settings.title} wide />
      <div className="pagebody wide" ref={scroller}>
        <div className="settings">
          <nav className="setnav" aria-label={t.settings.sections}>
            {groupKeys.map((k) => (
              <button key={k} type="button" className="setnav-item" aria-current={current === k ? 'true' : undefined} onClick={() => jump(k)}>
                {t.settings.groups[k]}
              </button>
            ))}
          </nav>
          <div className="setcol">
            {group('general', <>
              <section className="card pad">
                <h3>{t.settings.themeTitle}</h3>
                <div className="seg" role="group" aria-label={t.settings.themeTitle}>
                  {themes.map(([c, label, icon]) => (
                    <button key={c} type="button" aria-pressed={theme === c} onClick={() => onTheme(c)}><Icon name={icon} className="icon sm" />{label}</button>
                  ))}
                </div>
              </section>
              <section className="card pad">
                <h3>{t.settings.languageTitle}</h3>
                <div className="seg" role="group" aria-label={t.settings.languageTitle}>
                  <button type="button" aria-pressed={lang === 'en'} onClick={() => onLanguage('en')}>{t.settings.languageEn}</button>
                  <button type="button" aria-pressed={lang === 'vi'} onClick={() => onLanguage('vi')}>{t.settings.languageVi}</button>
                </div>
              </section>
              <section className="card pad">
                <h3>{t.onOpen.title}</h3>
                <OnOpenSettings />
              </section>
            </>)}

            {group('profiles', <>
              <section className="card pad">
                <h3>{t.defaults.title}</h3>
                <DefaultsSettings />
              </section>
              <section className="card pad">
                <h3>{t.sites.customTitle}</h3>
                <p className="desc">{t.sites.customDesc}</p>
                <SiteSettings sites={sites} reload={reloadSites} />
              </section>
              <section className="card pad" id="settings-extensions">
                <h3>{t.extensions.title}</h3>
                <ExtensionSettings />
              </section>
            </>)}

            {group('automation', <>
              <section className="card pad" id="settings-pro">
                <h3>{t.pro.title}</h3>
                <ProSettings />
              </section>
              <section className="card pad">
                <h3>{t.api.title}</h3>
                <APISettings />
              </section>
            </>)}

            {group('data', <>
              <section className="card pad" id="settings-backup">
                <h3>{t.backup.title}</h3>
                <BackupSettings />
              </section>
              <section className="card pad">
                <h3>{t.storage.title}</h3>
                <StorageSettings />
              </section>
              <section className="card pad">
                <h3>{t.settings.securityTitle}</h3>
                <div className={`banner ${fileVaultOn ? 'ok' : 'warn'}`}><Icon name="shield" />
                  <span className="grow">{fileVaultOn ? t.settings.fileVaultOn : t.settings.fileVaultOff}</span></div>
                <p className="desc" style={{ margin: 0 }}>{t.settings.secretsNote}</p>
              </section>
            </>)}

            {group('engines', (
              <section className="card pad">
                <h3 className="sr-only">{t.engines.title}</h3>
                <EngineSettings engines={engines} counts={engineCounts} defaults={defaults} onChanged={onEnginesChanged} />
              </section>
            ))}

            {group('about', <>
              <section className="card pad" id="settings-update">
                <h3>{t.update.title}</h3>
                <UpdateSettings openProfiles={openProfiles} checkToken={checkUpdate} />
              </section>
              {shownDonateLinks().length > 0 && (
                <section className="card pad">
                  <h3>{t.donate.title}</h3>
                  <p className="desc">{t.donate.desc}</p>
                  <DonateSettings />
                </section>
              )}
              <section className="card pad">
                <h3>{t.support.title}</h3>
                <SupportSettings />
              </section>
            </>)}
          </div>
        </div>
      </div>
    </>
  )
}
